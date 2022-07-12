package input

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"

	"sync"
	"time"

	"github.com/devopsext/events/common"

	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"

	"github.com/embano1/waitgroup"
	"github.com/jpillora/backoff"

	"github.com/vmware/govmomi"
	vcevent "github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	vctypes "github.com/vmware/govmomi/vim25/types"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type Option func(e *cloudevents.Event) error

type VCInputOptions struct {
	URL           string
	InsecureSSL   bool
	Checkpoint    bool
	AuthType      string
	AuthBasicName string
	AuthBasicPass string
	RootCA        string
	CheckpointDir string
	DelayMS       int
	//	PollDelayMS   time.Duration
}

type VCInput struct {
	options      VCInputOptions
	client       *govmomi.Client
	ctx          context.Context
	processors   *common.Processors
	tracer       sreCommon.Tracer
	logger       sreCommon.Logger
	meter        sreCommon.Meter
	requests     sreCommon.Counter
	errors       sreCommon.Counter
	wg           waitgroup.WaitGroup
	ceAttributes map[string]string // custom cloudevent context attributes added to events
}

type vcLastEvent struct {
	baseEvent vctypes.BaseEvent
	uuid      string
	key       int32
}

type vcEventInfo struct {
	Category string
	Name     string
}

var (
	errInvalidEvent     = errors.New("invalid event")
	errInvalidProcessor = errors.New("no processor found")
)

const (
	EventCanonicalType    = "com.vmware.event.router"
	EventSpecVersion      = cloudevents.VersionV1
	EventContentType      = cloudevents.ApplicationJSON
	format                = "cp-%s.json" // cp-<vc_hostname>.json
	eventsPageMax         = 100          // events per page from history collector
	checkpointInterval    = 5 * time.Second
	checkpointMaxEventAge = time.Hour           // limit event replay time window to max
	ceVSphereAPIKey       = "vsphereapiversion" // extended attribute representing vSphere API version
)

func newHistoryCollector(ctx context.Context, vcClient *vim25.Client, begin *time.Time) (*vcevent.HistoryCollector, error) {
	mgr := vcevent.NewManager(vcClient)
	root := vcClient.ServiceContent.RootFolder

	vcFilter := vctypes.EventFilterSpec{
		Entity: &vctypes.EventFilterSpecByEntity{
			Entity:    root,
			Recursion: vctypes.EventFilterSpecRecursionOptionAll,
		},
		Time: &vctypes.EventFilterSpecByTime{
			BeginTime: vctypes.NewTime(*begin),
		},
	}

	return mgr.CreateCollectorForEvents(ctx, vcFilter)
}

// checkpoint represents a checkpoint object from VEBA project https://github.com/vmware-samples/vcenter-event-broker-appliance
type checkpoint struct {
	// checkpoint to vc mapping
	VCenter string `json:"vCenter"`
	// last event UUID
	LastEventUUID string `json:"lastEventUUID"`
	// last vCenter event key successfully processed
	LastEventKey int32 `json:"lastEventKey"`
	// last event type, e.g. VmPoweredOffEvent useful for debugging
	LastEventType string `json:"lastEventType"`
	// last vCenter event key timestamp (UTC) successfully processed - used for
	// replaying the event history
	LastEventKeyTimestamp time.Time `json:"lastEventKeyTimestamp"`
	// timestamp (UTC) when this checkpoint was created
	CreatedTimestamp time.Time `json:"createdTimestamp"`
}

// getCheckpoint returns a checkpoint and its full path for the given host and
// directory. If no existing checkpoint is found, an empty checkpoint and
// associated file is created. Thus, the checkpoint might be in initialized state
// (i.e. default values) and it is the caller's responsibility to check for
// validity using time.IsZero() on any timestamp.
func getCheckpoint(ctx context.Context, host, dir string) (cp *checkpoint, path string, err error) {
	var (
		skip bool
	)

	file := fmt.Sprintf(format, host)
	path = fullPath(file, dir)

	f, err := os.Open(path)
	defer func() {
		if f != nil {
			closeWithErrCapture(&err, f, "could not close file")
		}
	}()

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			skip = true
			cp, err = initCheckpoint(ctx, path)
			if err != nil {
				return nil, "", errors.Wrap(err, "could not initialize checkpoint")
			}
		} else {
			return nil, "", errors.Wrap(err, "could not configure checkpointing")
		}
	}

	if !skip {
		cp, err = lastCheckpoint(ctx, f)
		if err != nil {
			return nil, "", errors.Wrap(err, "could not retrieve last checkpoint")
		}
	}

	return cp, path, nil
}

// initCheckpoint creates an empty checkpoint file with default values at the
// given full file path and returns the checkpoint.
func initCheckpoint(_ context.Context, fullPath string) (*checkpoint, error) {
	dir := filepath.Dir(fullPath)

	// create if not exists
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, errors.Wrap(err, "could not create checkpoint directory")
	}

	// create empty checkpoint
	var cp checkpoint
	jsonBytes, err := json.Marshal(cp)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal checkpoint to JSON object")
	}

	err = ioutil.WriteFile(fullPath, jsonBytes, 0600)
	if err != nil {
		return nil, errors.Wrap(err, "could not write checkpoint file")
	}
	return &cp, nil
}

// lastCheckpoint returns the last checkpoint for the given file
func lastCheckpoint(_ context.Context, file io.Reader) (*checkpoint, error) {
	var cp checkpoint
	jsonBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, "could not read checkpoint file")
	}

	if err = json.Unmarshal(jsonBytes, &cp); err != nil {
		return nil, errors.Wrap(err, "could not validate last checkpoint")
	}

	return &cp, nil
}

// createCheckpoint creates a checkpoint using the given file, vcenter host name
// and checkpoint timestamp returning the created checkpoint. If lastEvent is
// nil an errInvalidEvent will be returned.
func createCheckpoint(_ context.Context, file io.Writer, vcHost string, last vcLastEvent, timestamp time.Time) (*checkpoint, error) {
	be := last.baseEvent

	// will panic when the baseEvent value is not pointer
	if be == nil || reflect.ValueOf(be).IsNil() {
		return nil, errInvalidEvent
	}

	createdTime := be.GetEvent().CreatedTime
	eventDetails := getDetails(be)

	cp := checkpoint{
		VCenter:               vcHost,
		LastEventUUID:         last.uuid,
		LastEventKey:          last.key,
		LastEventType:         eventDetails.Name,
		LastEventKeyTimestamp: createdTime,
		CreatedTimestamp:      timestamp,
	}

	b, err := json.Marshal(cp)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal checkpoint to JSON")
	}

	_, err = file.Write(b)
	if err != nil {
		return nil, errors.Wrap(err, "could not write checkpoint")
	}
	return &cp, nil
}

// fullPath returns the full path for the given checkpoint file name and
// directory
func fullPath(file, dir string) string {
	dir = filepath.Clean(dir)
	return filepath.Join(dir, file) // file: e.g. checkpoints/cp-<hostname>.json
}

// closeWithErrCapture runs function and on error return error by argument including the given error (usually
// from caller function).
func closeWithErrCapture(err *error, closer io.Closer, errMsg string) {
	*err = errors.Wrapf(closer.Close(), errMsg)
}

func getDetails(event vctypes.BaseEvent) vcEventInfo {
	var eventInfo vcEventInfo

	switch e := event.(type) {
	case *vctypes.EventEx:
		eventInfo.Category = "eventex"
		eventInfo.Name = e.EventTypeId
	case *vctypes.ExtendedEvent:
		eventInfo.Category = "extendedevent"
		eventInfo.Name = e.EventTypeId

	default:
		eType := reflect.TypeOf(event).Elem().Name()
		eventInfo.Category = "event"
		eventInfo.Name = eType
	}

	return eventInfo
}

// NewFromVSphere returns a compliant CloudEvent for the given vSphere event
func NewFromVSphere(vcEvent vctypes.BaseEvent, source string, options ...Option) (*cloudevents.Event, error) {
	vcEventInfo := getDetails(vcEvent)
	ce := cloudevents.NewEvent(EventSpecVersion)

	// URI of the event producer, e.g. http(s)://vcenter.domain.ext/sdk
	ce.SetSource(source)

	// apply defaults
	ce.SetID(uuid.New().String())
	ce.SetTime(vcEvent.GetEvent().CreatedTime)

	ce.SetType(EventCanonicalType + "/" + vcEventInfo.Category)
	ce.SetSubject(vcEventInfo.Name)

	var err error
	err = ce.SetData(EventContentType, vcEvent)
	if err != nil {
		return nil, errors.Wrap(err, "set CloudEvent data")
	}

	// apply options
	for _, opt := range options {
		if err = opt(&ce); err != nil {
			return nil, errors.Wrap(err, "apply option")
		}
	}

	if err = ce.Validate(); err != nil {
		return nil, errors.Wrap(err, "validation for CloudEvent failed")
	}

	return &ce, nil
}
func WithAttributes(ceAttrs map[string]string) Option {
	return func(e *cloudevents.Event) error {
		for k, v := range ceAttrs {
			e.SetExtension(k, v)
		}
		return nil
	}
}

//

func newClient(ctx context.Context, u *url.URL, rootCA string, insecure bool) (*govmomi.Client, error) {
	soapClient := soap.NewClient(u, insecure)
	err := soapClient.SetRootCAs(rootCA)
	if err != nil {
		return nil, err
	}

	vsClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	c := &govmomi.Client{
		Client:         vsClient,
		SessionManager: session.NewManager(vsClient),
	}

	if err = c.Login(ctx, u.User); err != nil {
		return nil, err
	}
	return c, nil
}

/////////

func NewVCInput(options VCInputOptions, processors *common.Processors, observability *common.Observability) *VCInput {

	logger := observability.Logs()

	//vcStream := EventStream{
	//	ceAttributes: make(map[string]string),
	//}
	if utils.IsEmpty(options.URL) || utils.IsEmpty(options.AuthBasicName) || utils.IsEmpty(options.AuthBasicPass) {
		logger.Debug("VC input credentials or URL is not defined. Skipped")
		return nil
	}
	parsedURL, err := soap.ParseURL(options.URL)
	if err != nil {
		logger.Debug(err, "error parsing vCenter URL:")
		return nil
	}
	parsedURL.User = url.UserPassword(options.AuthBasicName, options.AuthBasicPass)

	ctx := context.Background()
	client, err := newClient(ctx, parsedURL, options.RootCA, options.InsecureSSL)
	if err != nil {
		logger.Debug(err, "create client")
		return nil

	}

	ceAttributes := make(map[string]string)
	ceAttributes[ceVSphereAPIKey] = client.ServiceContent.About.ApiVersion

	meter := observability.Metrics()
	return &VCInput{
		options:      options,
		client:       client,
		ctx:          ctx,
		processors:   processors,
		tracer:       observability.Traces(),
		logger:       observability.Logs(),
		meter:        meter,
		requests:     meter.Counter("requests", "Count of all vc input requests", []string{"url"}, "vcenter", "input"),
		errors:       meter.Counter("errors", "Count of all vc input errors", []string{"url"}, "vcenter", "input"),
		ceAttributes: ceAttributes,
	}
}
func (vc *VCInput) Start(wg *sync.WaitGroup, _ *common.Outputs) {
	var (
		begin *time.Time
		cp    *checkpoint
		path  string
		err   error
	)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		vc.logger.Info("Start vcenter input...")

		// begin of event stream defaults to current vCenter time (UTC)
		begin, err = methods.GetCurrentTime(vc.ctx, vc.client)
		if err != nil {
			vc.logger.Error(err, "get current time from vCenter")
		}

		// configure checkpointing and retrieve last checkpoint, if any
		switch vc.options.Checkpoint {
		case true:
			vc.logger.Debug("enabling checkpoints and checking for existing checkpoint")

			cp, path, err = getCheckpoint(vc.ctx, vc.client.URL().Hostname(), vc.options.CheckpointDir)

			// if the timestamp is valid set begin to last checkpoint
			ts := cp.LastEventKeyTimestamp
			if !ts.IsZero() {
				vc.logger.Debug("found existing and valid checkpoint", "path", path)
				// perform boundary check
				maxTS := begin.Add(checkpointMaxEventAge * -1)
				if maxTS.Unix() > ts.Unix() {
					begin = &maxTS
					vc.logger.Debug("last event timestamp in checkpoint is older than configured maximum", "maxTimestamp", checkpointMaxEventAge.String())
					vc.logger.Debug("setting begin of event stream", "beginTimestamp", begin.String())
				} else {
					begin = &ts
					vc.logger.Debug("setting begin of event stream", "beginTimestamp", begin.String(), "eventKey", cp.LastEventKey)
				}
			} else {
				vc.logger.Debug("no valid checkpoint found")
				vc.logger.Debug("empty checkpoint created", "path", path)
				vc.logger.Debug("setting begin of event stream", "beginTimestamp", begin.String())
			}

		case false:
			vc.logger.Debug("checkpointing disabled, setting begin of event stream", "beginTimestamp", begin.String())
		}

		ehc, err := newHistoryCollector(vc.ctx, vc.client.Client, begin)
		if err != nil {
			vc.logger.Error(err, "create event history collector")
		}

		defer func() {
			// use new ctx bc current might be cancelled
			if vc.ctx.Err() != nil {
				vc.ctx = context.Background()
			}
			_ = ehc.Destroy(vc.ctx) // ignore any err
		}()

		vc.wg.Add(1)
		defer vc.wg.Done()
		err = vc.pollEvents(ehc)
		if err != nil {
			vc.logger.Error(err, "errors with event history collector polling")
		}
	}(wg)
}

func (vc *VCInput) pollEvents(collector *vcevent.HistoryCollector) error {
	// event poll ticker

	//	pollTick := time.NewTicker(vc.options.PollDelayMS * time.Millisecond)
	pollTick := time.NewTicker(time.Duration(vc.options.DelayMS) * time.Millisecond)
	defer pollTick.Stop()

	// create checkpoint ticker only if needed
	var cpTick <-chan time.Time = nil
	if vc.options.Checkpoint {
		cpTicker := time.NewTicker(checkpointInterval)
		cpTick = cpTicker.C
		defer cpTicker.Stop()
	}

	var (
		last      *vcLastEvent // last processed event
		lastCpKey int32        // last event key in checkpoint
		bOff      = backoff.Backoff{
			Factor: 2,
			Jitter: false,
			Min:    time.Second,
			Max:    5 * time.Second,
		}
	)

	for {
		select {
		case <-vc.ctx.Done():
			return vc.ctx.Err()

		case <-cpTick:
			// skip if checkpoint channel fires before first event or no new events received
			// since last checkpoint
			if last == nil || (last.key == lastCpKey) {
				vc.logger.Debug("no new events, skipping checkpoint")
				continue
			}

			host := vc.client.URL().Hostname()
			f := fmt.Sprintf(format, host)

			path := fullPath(f, vc.options.CheckpointDir)

			// always create/overwrite (existing) checkpoint
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return errors.Wrap(err, "create checkpoint file")
			}

			cp, err := createCheckpoint(vc.ctx, file, host, *last, time.Now().UTC())
			if err != nil {
				return errors.Wrap(err, "create checkpoint")
			}
			lastCpKey = cp.LastEventKey

			err = file.Close()
			if err != nil {
				return errors.Wrap(err, "close checkpoint file")
			}

			vc.logger.Debug("created checkpoint", "path", path, "eventKey", lastCpKey)

		case <-pollTick.C:
			vcBaseEvents, err := collector.ReadNextEvents(vc.ctx, eventsPageMax)

			if err != nil {
				return errors.Wrap(err, "retrieve events")
			}

			if len(vcBaseEvents) == 0 {
				sleep := bOff.Duration()
				vc.logger.Debug("no new events, backing off", "delaySeconds", sleep)
				time.Sleep(sleep)
				continue
			}

			last, err = vc.processEvents(vcBaseEvents)
			if err != nil {
				return errors.Wrap(err, "sending events")
			}
			bOff.Reset()
		}
	}
}

func (vc *VCInput) processEvents(vcBaseEvents []vctypes.BaseEvent) (*vcLastEvent, error) {
	var (
		errCount int
		last     *vcLastEvent
	)

	for _, e := range vcBaseEvents {

		ce, err := NewFromVSphere(e, vc.client.URL().String(), WithAttributes(vc.ceAttributes))
		if err != nil {
			vc.logger.Error("skipping event because it could not be converted to CloudEvent format", "event", e, "error", err)
			errCount++
			continue
		}

		vc.logger.Debug("invoking processor", "eventID", ce.ID())

		jsonBytes, err := json.Marshal(ce)
		if err != nil {
			vc.logger.Error("skipping cause couldnt Marchall", "jsonBytes", jsonBytes, "error", err)
			errCount++
			continue
		}

		curevent := &common.Event{
			Data:    string(jsonBytes),
			Channel: vc.client.URL().Hostname(),
			Type:    "vcenterEvent",
		}
		curevent.SetTime(time.Now().UTC())
		curevent.SetLogger(vc.logger)

		p := vc.processors.Find(curevent.Type)
		if p == nil {
			vc.logger.Debug("VC processor is not found for %s", curevent.Type)
			return nil, errInvalidProcessor
		}
		vc.requests.Inc(curevent.Channel)
		err = p.HandleEvent(curevent)
		if err != nil {
			vc.logger.Debug("some problems with %v", err)
			vc.errors.Inc(curevent.Channel)
		}

		last = &vcLastEvent{
			baseEvent: e,
			uuid:      ce.ID(),
			key:       e.GetEvent().Key,
		}
	}

	return last, nil
}
