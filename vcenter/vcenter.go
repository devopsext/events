package vcenter

import (
	"context"
	"encoding/json"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/output"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/embano1/waitgroup"
	"github.com/jpillora/backoff"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sync"
	"time"
)

const (
	defaultPollFrequency  = time.Second
	eventsPageMax         = 100 // events per page from history collector
	checkpointInterval    = 5 * time.Second
	checkpointMaxEventAge = time.Hour           // limit event replay time window to max
	waitShutdown          = 5 * time.Second     // wait for processing to finish during shutdown
	ceVSphereAPIKey       = "vsphereapiversion" // extended attribute representing vSphere API version
)

// EventStream handles the connection to the vCenter events API
type EventStream struct {
	client        *govmomi.Client
	logger        sreCommon.Logger
	checkpoint    bool
	checkpointDir string
	rootCAs       []string          // custom root CAs, TLS OS defaults if not specified
	ceAttributes  map[string]string // custom cloudevent context attributes added to events

	wg waitgroup.WaitGroup // shutdown handling

	sync.RWMutex
}

type lastEvent struct {
	baseEvent types.BaseEvent
	uuid      string
	key       int32
}

type ProviderConfigVCenter struct {
	// Address of the vCenter server (URI)
	Address string `yaml:"address" json:"address" jsonschema:"required,default=https://my-vcenter01.domain.local/sdk"`
	// InsecureSSL enables/disables TLS certificate validation
	InsecureSSL bool `yaml:"insecureSSL" json:"insecureSSL" jsonschema:"required,default=false"`
	// Checkpoint enables/disables event replay from a checkpoint file
	Checkpoint bool `yaml:"checkpoint" json:"checkpoint" jsonschema:"description=Enable checkpointing via checkpoint file for event recovery and replay purposes"`
	// CheckpointDir sets the directory for persisting checkpoints (optional)
	CheckpointDir string `yaml:"checkpointDir,omitempty" json:"checkpointDir,omitempty" jsonschema:"description=Directory where to persist checkpoints if enabled,default=./checkpoints"`
	// Auth sets the vCenter authentication credentials. Only basic_auth is
	// supported.
	Certificates []string
	//Auth         *VEBAconfig.AuthMethod `yaml:"auth,omitempty" json:"auth,omitempty" jsonschema:"oneof_required=auth,description=Authentication configuration for this section"`
}

// VCInputOptions NewEventStream returns a vCenter event stream manager for a given
// configuration and metrics server
type VCInputOptions struct {
	URL           string
	InsecureSSL   bool
	Checkpoint    bool
	AuthType      string
	AuthBasicName string
	AuthBasicPass string
	RootCA        string
	CheckpointDir string
}

func NewEventStream(ctx context.Context, logger sreCommon.Logger, options VCInputOptions) (*EventStream, error) {
	//if cfg == nil {
	//	return nil, errors.New("vCenter configuration must be provided")
	//}
	var cfg ProviderConfigVCenter
	cfg.Address = options.URL

	vcStream := EventStream{
		ceAttributes: make(map[string]string),
	}
	vcStream.logger = logger

	parsedURL, err := soap.ParseURL(options.URL)
	if err != nil {
		return nil, errors.Wrap(err, "parsing vCenter URL")
	}

	username := options.AuthBasicName
	password := options.AuthBasicPass
	parsedURL.User = url.UserPassword(username, password)

	if vcStream.client == nil {
		vcStream.client, err = newClient(ctx, parsedURL, options.RootCA, options.InsecureSSL)
		if err != nil {
			return nil, errors.Wrap(err, "create client")
		}
	}
	vcStream.ceAttributes[ceVSphereAPIKey] = vcStream.client.ServiceContent.About.ApiVersion

	vcStream.checkpoint = true
	vcStream.checkpointDir = options.CheckpointDir

	return &vcStream, nil
}

func newClient(ctx context.Context, u *url.URL, rootCA string, insecure bool) (*govmomi.Client, error) {

	soapClient := soap.NewClient(u, insecure)

	err := soapClient.SetRootCAs(rootCA)
	if err != nil {
		return nil, err
	}

	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	c := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	if err = c.Login(ctx, u.User); err != nil {
		return nil, err
	}
	return c, nil
}

// Stream is the main logic, blocking to receive and handle events from vCenter
func (vc *EventStream) Stream(ctx context.Context, outputs *common.Outputs) error {
	var (
		begin *time.Time
		cp    *checkpoint
		path  string
		err   error
	)

	// begin of event stream defaults to current vCenter time (UTC)
	begin, err = methods.GetCurrentTime(ctx, vc.client)
	if err != nil {
		return errors.Wrap(err, "get current time from vCenter")
	}

	// configure checkpointing and retrieve last checkpoint, if any
	switch vc.checkpoint {
	case true:
		vc.logger.Debug("enabling checkpoints and checking for existing checkpoint")
		host := vc.client.URL().Hostname()

		dir := defaultCheckpointDir
		if vc.checkpointDir != "" {
			dir = vc.checkpointDir
		}

		cp, path, err = getCheckpoint(ctx, host, dir)
		if err != nil {
			return errors.Wrap(err, "get checkpoint")
		}

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

	ehc, err := newHistoryCollector(ctx, vc.client.Client, begin)
	if err != nil {
		return errors.Wrap(err, "create event history collector")
	}

	defer func() {
		// use new ctx bc current might be cancelled
		if ctx.Err() != nil {
			ctx = context.Background()
		}
		_ = ehc.Destroy(ctx) // ignore any err
	}()

	vc.wg.Add(1)
	defer vc.wg.Done()
	return vc.stream(ctx, ehc, vc.checkpoint, outputs)
}

func (vc *EventStream) stream(ctx context.Context, collector *event.HistoryCollector, enableCheckpoint bool, outputs *common.Outputs) error {
	// event poll ticker
	pollTick := time.NewTicker(defaultPollFrequency)
	defer pollTick.Stop()

	// create checkpoint ticker only if needed
	var cpTick <-chan time.Time = nil
	if enableCheckpoint {
		cpTicker := time.NewTicker(checkpointInterval)
		cpTick = cpTicker.C
		defer cpTicker.Stop()
	}

	var (
		last      *lastEvent // last processed event
		lastCpKey int32      // last event key in checkpoint
		bOff      = backoff.Backoff{
			Factor: 2,
			Jitter: false,
			Min:    time.Second,
			Max:    5 * time.Second,
		}
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		// 	there is a small chance (timing and channel handling) that we received
		// 	event(s) and crashed before creating the first checkpoint. at-least-once
		// 	would be violated because we come back with an empty initialized checkpoint.
		// 	TODO: we could force a checkpoint after the first event to reduce the likelihood
		case <-cpTick:

			// skip if checkpoint channel fires before first event or no new events received
			// since last checkpoint
			if last == nil || (last.key == lastCpKey) {
				vc.logger.Debug("no new events, skipping checkpoint")
				continue
			}

			host := vc.client.URL().Hostname()
			f := fileName(host)

			dir := defaultCheckpointDir
			if vc.checkpointDir != "" {
				dir = vc.checkpointDir
			}
			path := fullPath(f, dir)

			// always create/overwrite (existing) checkpoint
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return errors.Wrap(err, "create checkpoint file")
			}

			cp, err := createCheckpoint(ctx, file, host, *last, time.Now().UTC())
			if err != nil {
				return errors.Wrap(err, "create checkpoint")
			}
			lastCpKey = cp.LastEventKey

			err = file.Close()
			if err != nil {
				return errors.Wrap(err, "close checkpoint file")
			}

			vc.logger.Info("created checkpoint", "path", path, "eventKey", lastCpKey)

		case <-pollTick.C:
			baseEvents, err := collector.ReadNextEvents(ctx, eventsPageMax)
			// TODO: handle error without returning?
			if err != nil {
				return errors.Wrap(err, "retrieve events")
			}

			if len(baseEvents) == 0 {
				sleep := bOff.Duration()
				vc.logger.Debug("no new events, backing off", "delaySeconds", sleep)
				time.Sleep(sleep)
				continue
			}

			last = vc.processEvents(baseEvents, outputs)
			bOff.Reset()
		}
	}
}

// processEvents processes events from vcenter serially, i.e. in order, invoking
// the supplied processor. Errors are logged and tracked in the metric stats.
// The last event processed, including those returning with error, is returned.
func (vc *EventStream) processEvents(baseEvents []types.BaseEvent, outputs *common.Outputs) *lastEvent {
	var (
		errCount int
		last     *lastEvent
	)

	host := vc.client.URL().String()

	for _, e := range baseEvents {
		ce, err := NewFromVSphere(e, host, WithAttributes(vc.ceAttributes))
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

		jsonString := string(jsonBytes)
		var result map[string]interface{}
		json.Unmarshal([]byte(jsonString), &result)

		var k int
		for i, v := range outputs.List {
			if reflect.TypeOf(v).String() == reflect.TypeOf((*output.TelegramOutput)(nil)).String() {
				k = i
				break
			}
		}
		tgout := outputs.List[k]

		eventSubject := result["subject"].(string)
		eventChannel := "vcenter"
		var eventData string
		data := result["data"].(map[string]interface{})
		for key, value := range data {
			if key == "FullFormattedMessage" {
				eventData = value.(string)
			}
		}
		eventTime, _ := time.Parse(time.RFC3339Nano, result["time"].(string))
		curevent := &common.Event{
			Data:    eventData,
			Time:    eventTime,
			Channel: eventChannel,
			Type:    eventSubject,
		}

		regexpString := "AlarmStatusChangedEvent" +
			"|" + "com.vmware.vc.sdrs.ClearDatastoreInMultipleDatacentersEvent" +
			"|" + "AlarmStatusChangedEvent" +
			"|" + "TaskEvent" +
			"|" + "NoAccessUserEvent" +
			"|" + "UserLogoutSessionEvent" +
			"|" + "UserLoginSessionEvent"

		r, _ := regexp.Compile(regexpString)
		if !r.MatchString(eventSubject) {
			tgout.Send(curevent)
		}

		last = &lastEvent{
			baseEvent: e,
			uuid:      ce.ID(),
			key:       e.GetEvent().Key,
		}
	}

	// update metrics
	vc.Lock()
	//total := *vc.stats.EventsTotal + len(baseEvents)
	//vc.stats.EventsTotal = &total
	//errTotal := *vc.stats.EventsErr + errCount
	//vc.stats.EventsErr = &errTotal
	vc.Unlock()

	return last
}

// Shutdown closes the underlying connection to vCenter
func (vc *EventStream) Shutdown(ctx context.Context) error {
	vc.logger.Info("attempting graceful shutdown")
	if err := vc.wg.WaitTimeout(waitShutdown); err != nil {
		return errors.Wrap(err, "shutdown")
	}

	// create new ctx in case current already cancelled
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	return errors.Wrap(vc.client.Logout(ctx), "logout from vCenter") // err == nil if logout was successful
}

func newHistoryCollector(ctx context.Context, vcClient *vim25.Client, begin *time.Time) (*event.HistoryCollector, error) {
	mgr := event.NewManager(vcClient)
	root := vcClient.ServiceContent.RootFolder

	// configure the event stream filter (begin of stream)
	filter := types.EventFilterSpec{
		// EventTypeId: []string{...}, // only stream specific types, e.g. VmEvent
		Entity: &types.EventFilterSpecByEntity{
			Entity:    root,
			Recursion: types.EventFilterSpecRecursionOptionAll,
		},
		Time: &types.EventFilterSpecByTime{
			BeginTime: types.NewTime(*begin),
		},
	}

	return mgr.CreateCollectorForEvents(ctx, filter)
}
