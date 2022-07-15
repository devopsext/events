package processor

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type VCenterProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type VCenterResponse struct {
	Message string
}

func VCenterProcessorType() string {
	return "VCenter"
}

func (p *VCenterProcessor) EventType() string {
	return common.AsEventType(VCenterProcessorType())
}

func (p *VCenterProcessor) prepareStatus(status string) string {
	return strings.Title(strings.ToLower(status))
}

func (p *VCenterProcessor) send(span sreCommon.TracerSpan, channel string, data string) {

	for _, alert := range data {

		e := &common.Event{
			Channel: channel,
			Type:    p.EventType(),
			Data:    alert,
		}
		//e.SetTime(alert..UTC())
		if span != nil {
			e.SetSpanContext(span.GetContext())
			e.SetLogger(p.logger)
		}
		p.outputs.Send(e)
	}
}

var ErrorEventNotImpemented = errors.New("vcenter event not implemented yet")

type vcenterEvent struct {
	Subject            string
	CreatedTime        string
	FullUsername       string
	Message            string
	VmName             string
	OrigClusterName    string
	OrigDatacenterName string
	OrigLocation       string
	OrigESXiHostName   string
	OrigDatastoreName  string
	DestClusterName    string
	DestDatacenterName string
	DestLocation       string
	DestESXiHostName   string
	DestDatastoreName  string
	EntityName         string
	EntityType         string
	AlarmName          string
	Argument1          string
	Argument2          string
}

func (vce *vcenterEvent) parse(jsonByte []byte) error {
	vce.CreatedTime = " "
	vce.FullUsername = " "
	vce.Message = " "
	vce.VmName = " "
	vce.OrigClusterName = " "
	vce.OrigDatacenterName = " "
	vce.OrigLocation = " "
	vce.OrigESXiHostName = " "
	vce.OrigDatastoreName = " "
	vce.DestClusterName = " "
	vce.DestDatacenterName = " "
	vce.DestLocation = " "
	vce.DestESXiHostName = " "
	vce.DestDatastoreName = " "
	vce.EntityName = " "
	vce.EntityType = " "
	vce.AlarmName = " "
	vce.Argument1 = " "
	vce.Argument2 = " "
	var err error

	// common part
	vce.CreatedTime, _ = jsonparser.GetString(jsonByte, "data", "CreatedTime")
	vce.Message, _ = jsonparser.GetString(jsonByte, "data", "FullFormattedMessage")
	vce.FullUsername, _ = jsonparser.GetString(jsonByte, "data", "UserName")
	if vce.VmName, err = jsonparser.GetString(jsonByte, "data", "Vm", "Name"); err != nil {
		vce.VmName = " "
	}

	// fields filling and return
	switch vce.Subject {
	//esx
	case "esx.problem.vmfs.heartbeat.recovered",
		"esx.audit.vmfs.sesparse.bloomfilter.disabled",
		"esx.clear.scsi.device.io.latency.improved",
		"esx.problem.clock.correction.adjtime.lostsync",
		"esx.problem.clock.correction.adjtime.sync",
		"esx.problem.visorfs.ramdisk.full",
		"esx.problem.vmfs.heartbeat.timedout":
		vce.DestClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.DestDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
		vce.DestESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "Host", "Name")

		jsonparser.ArrayEach(jsonByte, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			key, _ := jsonparser.GetString(value, "Key")
			switch key {
			case "1":
				vce.Argument1, _ = jsonparser.GetString(value, "Value")
			case "2":
				vce.Argument2, _ = jsonparser.GetString(value, "Value")
			}
		}, "data", "Arguments")
		return nil
	//Vm
	case "VmPoweredOffEvent",
		"VmAcquiredTicketEvent",
		"VmBeingClonedEvent",
		"VmBeingCreatedEvent",
		"VmBeingDeployedEvent",
		"VmBeingHotMigratedEvent",
		"VmBeingRelocatedEvent",
		"VmCloneFailedEvent",
		"VmClonedEvent",
		"VmConfigMissingEvent",
		"VmConnectedEvent",
		"VmCreatedEvent",
		"VmDeployedEvent",
		"VmDisconnectedEvent",
		"VmEmigratingEvent",
		"VmFailedMigrateEvent",
		"VmGuestShutdownEvent",
		"VmInstanceUuidAssignedEvent",
		"VmMacAssignedEvent",
		"VmMacChangedEvent",
		"VmMessageErrorEvent",
		"VmMessageEvent",
		"VmMigratedEvent",
		"VmPoweredOnEvent",
		"VmReconfiguredEvent",
		"VmRelocateFailedEvent",
		"VmRelocatedEvent",
		"VmRemovedEvent",
		"VmResettingEvent",
		"VmResourcePoolMovedEvent",
		"VmResourceReallocatedEvent",
		"VmStartingEvent",
		"VmStoppingEvent",
		"VmUuidAssignedEvent":
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.OrigClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
		vce.OrigDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
		vce.DestDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "DestDatacenter", "Datacenter", "Value")
		vce.OrigESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "Host", "Name")
		vce.DestESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "DestHost", "Name")

		return nil
	//com.vmware.vc.HA
	case "com.vmware.vc.HA.HostDasAgentHealthyEvent",
		"com.vmware.vc.HA.AllHostAddrsPingable",
		"com.vmware.vc.HA.ConnectedToMaster",
		"com.vmware.vc.HA.HostStateChangedEvent",
		"com.vmware.vc.HA.NotAllHostAddrsPingable",
		"com.vmware.vc.HA.VmProtectedEvent",
		"com.vmware.vc.HA.VmUnprotectedEvent":
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.DestClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
		vce.DestDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
		vce.DestESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "Host", "Name")
		return nil
	//com.vmware.vc.sdrs
	case "com.vmware.vc.sdrs.ClearDatastoreInMultipleDatacentersEvent",
		"com.vmware.vc.sdrs.StorageDrsNewRecommendationPendingEvent",
		"com.vmware.vc.sdrs.StorageDrsStorageMigrationEvent",
		"com.vmware.vc.sdrs.StorageDrsStoragePlacementEvent",
		"com.vmware.vc.sdrs.ConfiguredStorageDrsOnPodEvent":
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.DestClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
		vce.DestDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
		return nil
	//Drs
	case "DrsVmMigratedEvent",
		"DrsVmPoweredOnEvent":
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.OrigClusterName, _ = jsonparser.GetString(jsonByte, "data", "SourceDatacenter", "Name")
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.OrigESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "SourceHost", "Name")
		vce.OrigDatastoreName, _ = jsonparser.GetString(jsonByte, "data", "SourceDatastore", "Name")
		vce.DestClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
		vce.DestDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
		vce.DestESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "Host", "Name")
		vce.DestDatastoreName, _ = jsonparser.GetString(jsonByte, "data", "Ds", "Name")
		return nil
	//Dvs
	case "DvsPortConnectedEvent",
		"DvsPortCreatedEvent",
		"DvsPortDeletedEvent",
		"DvsPortDisconnectedEvent",
		"DvsPortExitedPassthruEvent",
		"DvsPortLinkDownEvent",
		"DvsPortLinkUpEvent",
		"DvsPortUnblockedEvent":
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.OrigClusterName, _ = jsonparser.GetString(jsonByte, "data", "SourceDatacenter", "Name")
		vce.OrigDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "SourceDatacenter", "Datacenter", "Value")
		vce.OrigESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "SourceHost", "Name")
		vce.OrigDatastoreName, _ = jsonparser.GetString(jsonByte, "data", "SourceDatastore", "Name")
		vce.DestClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
		vce.DestDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
		vce.DestESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "Host", "Name")
		vce.DestDatastoreName, _ = jsonparser.GetString(jsonByte, "data", "Ds", "Name")
		vce.Argument1, _ = jsonparser.GetString(jsonByte, "data", "Dvs", "Name")
		vce.Argument2, _ = jsonparser.GetString(jsonByte, "data", "PortKey")
		vce.VmName, err = jsonparser.GetString(jsonByte, "data", "Connectee", "ConnectedEntity", "Value")
		return nil
	//Alarm
	case "AlarmStatusChangedEvent",
		"AlarmActionTriggeredEvent",
		"AlarmEmailCompletedEvent",
		"AlarmSnmpCompletedEvent":
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.DestClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
		vce.DestDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
		vce.DestESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "Host", "Name")
		vce.EntityName, _ = jsonparser.GetString(jsonByte, "data", "Source", "Entity", "Value")
		vce.EntityType, _ = jsonparser.GetString(jsonByte, "data", "Source", "Entity", "Type")
		vce.AlarmName, _ = jsonparser.GetString(jsonByte, "data", "Alarm", "Name")
		return nil
	}

	// skip events and return nil

	switch vce.Subject {
	case "NoAccessUserEvent",
		"UserLogoutSessionEvent",
		"BadUsernameSessionEvent",
		"CustomFieldValueChangedEvent",
		"CustomizationStartedEvent",
		"CustomizationSucceeded",
		"DatastoreFileDeletedEvent",
		"DatastoreFileNfcModificationEvent",
		"DatastoreFileUploadEvent",
		"HostSyncFailedEvent",
		"ScheduledTaskCompletedEvent",
		"ScheduledTaskStartedEvent",
		"TaskEvent",
		"UserLoginSessionEvent",
		"GeneralHostWarningEvent",
		"com.vmware.pbm.profile.associate",
		"com.vmware.sso.LoginFailure",
		"com.vmware.sso.LoginSuccess",
		"com.vmware.sso.Logout",
		"com.vmware.vc.AllEventBurstsEndedEvent",
		"com.vmware.vc.EventBurstCompressedEvent",
		"com.vmware.vc.EventBurstEndedEvent",
		"com.vmware.vc.EventBurstStartedEvent",
		"com.vmware.vc.HardwareSensorGroupStatus",
		"com.vmware.vc.StatelessAlarmTriggeredEvent",
		"com.vmware.vc.VmDiskConsolidatedEvent",
		"com.vmware.vc.VmDiskConsolidationNeeded",
		"com.vmware.vc.authorization.NoPermission",
		"com.vmware.vc.host.Crypto.ReqEnable.KMSClusterError",
		"com.vmware.vc.vm.PMemBandwidthGreen",
		"com.vmware.vc.vm.SrcVmMigrateFailedEvent",
		"com.vmware.vc.vm.TemplateConvertedToVmEvent",
		"com.vmware.vc.vm.VmConvertedToTemplateEvent",
		"com.vmware.vc.vm.VmHotMigratingWithEncryptionEvent",
		"com.vmware.vc.vm.VmStateRevertedToSnapshot",
		"com.vmware.vc.vmam.VmAppHealthMonitoringStateChangedEvent",
		"com.vmware.vc.vmam.VmDasAppHeartbeatFailedEvent",
		"com.vmware.vcIntegrity.ScanStart",
		"com.vmware.vcIntegrity.ToolsScan",
		"com.vmware.vcIntegrity.VMHardwareUpgradeScan",
		"vsan.health.test.cluster.clustermembership.event",
		"vsan.health.test.data.objecthealth.event",
		"vsan.health.test.hcl.hclhostbadstate.event",
		"vsan.health.test.iscsi.iscsihomeobjectstatustest.event",
		"vsan.health.test.network.largeping.event",
		"vsan.health.test.network.smallping.event",
		"vsan.health.test.overallsummary.event",
		"vsan.health.test.perfsvc.hostsmissing.event",
		"GeneralUserEvent":
		vce.Subject = ""
		return nil
	}

	// not implemented event
	if _, err := os.Stat("test/vcenter/new/" + vce.Subject + ".json"); errors.Is(err, os.ErrNotExist) {
		f, _ := os.OpenFile("test/vcenter/new/"+vce.Subject+".json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		defer f.Close()
		fmt.Fprintf(f, "%s", jsonByte)
		fmt.Println("\t" + vce.Subject)
	}
	err = fmt.Errorf("%w", ErrorEventNotImpemented)
	return fmt.Errorf("%s %w", vce.Subject, err)
}

func (p *VCenterProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.errors.Inc("vcenter: Event is not defined")
		p.logger.Debug("Event is not defined")
		return nil
	}

	jsonString := e.Data.(string)
	subject, err := jsonparser.GetString([]byte(jsonString), "subject")
	if err != nil {
		p.errors.Inc("vcenter: No Subject")
		return err
	}
	vce := vcenterEvent{
		Subject: subject,
	}

	err = vce.parse([]byte(jsonString))
	if err != nil {
		p.errors.Inc("vcenter: Cannot parse")
		p.logger.Debug(err)
		return err
	}

	curevent := &common.Event{
		Data:    vce,
		Channel: e.Channel,
		Type:    "vcenterEvent",
	}
	curevent.SetLogger(p.logger)
	eventTime, _ := time.Parse(time.RFC3339Nano, vce.CreatedTime)

	curevent.SetTime(eventTime)

	p.requests.Inc(vce.Subject)
	p.outputs.Send(curevent)

	return nil
}
func NewVCenterProcessor(outputs *common.Outputs, observability *common.Observability) *VCenterProcessor {

	return &VCenterProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all vcenter processor requests", []string{"channel"}, "vcenter", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all vcenter processor errors", []string{"error"}, "vcenter", "processor"),
	}
}
