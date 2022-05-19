package vcenter

// allmost cloned from VEBA
// https://github.com/vmware-samples/vcenter-event-broker-appliance

import (
	"reflect"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	EventCanonicalType = "com.vmware.event.router"
	EventSpecVersion   = cloudevents.VersionV1
	EventContentType   = cloudevents.ApplicationJSON
)

type EventInfo struct {
	Category string
	Name     string
}

func GetDetails(event types.BaseEvent) EventInfo {
	var eventInfo EventInfo

	switch e := event.(type) {
	case *types.EventEx:
		eventInfo.Category = "eventex"
		eventInfo.Name = e.EventTypeId
	case *types.ExtendedEvent:
		eventInfo.Category = "extendedevent"
		eventInfo.Name = e.EventTypeId

	// TODO: make agnostic to vCenter events
	default:
		eType := reflect.TypeOf(event).Elem().Name()
		eventInfo.Category = "event"
		eventInfo.Name = eType
	}

	return eventInfo
}

type Option func(e *cloudevents.Event) error

func WithAttributes(ceAttrs map[string]string) Option {
	return func(e *cloudevents.Event) error {
		for k, v := range ceAttrs {
			e.SetExtension(k, v)
		}
		return nil
	}
}

// NewFromVSphere returns a compliant CloudEvent for the given vSphere event
func NewFromVSphere(event types.BaseEvent, source string, options ...Option) (*cloudevents.Event, error) {
	eventInfo := GetDetails(event)
	ce := cloudevents.NewEvent(EventSpecVersion)

	// URI of the event producer, e.g. http(s)://vcenter.domain.ext/sdk
	ce.SetSource(source)

	// apply defaults
	ce.SetID(uuid.New().String())
	ce.SetTime(event.GetEvent().CreatedTime)

	ce.SetType(EventCanonicalType + "/" + eventInfo.Category)
	ce.SetSubject(eventInfo.Name)

	var err error
	err = ce.SetData(EventContentType, event)
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
