package processor

import (
	errPkg "errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type VCenterProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
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

func (p *VCenterProcessor) send(channel string, data string) {

	for _, alert := range data {

		e := &common.Event{
			Channel: channel,
			Type:    p.EventType(),
			Data:    alert,
		}
		//e.SetTime(alert..UTC())
		e.SetLogger(p.logger)
		p.outputs.Send(e)
	}
}

var ErrorEventNotImpemented = errPkg.New("vcenter event not implemented yet")
var ErrorEventDefinitelySkip = errPkg.New("vcenter event for trash only")

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
	RAWString          string
	ChainId            string
	DebugSubject       string
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
	if vce.Message, err = jsonparser.GetString(jsonByte, "data", "FullFormattedMessage"); err != nil {
		vce.Message = " "
	}
	vce.FullUsername, _ = jsonparser.GetString(jsonByte, "data", "UserName")
	if vce.VmName, err = jsonparser.GetString(jsonByte, "data", "Vm", "Name"); err != nil {
		vce.VmName = " "
	}

	// fields filling and return
	switch vce.Subject {
	//definitly skip
	case "com.vmware.vc.authorization.NoPermission",
		"UserLogoutSessionEvent",
		"UserPasswordChanged",
		"VimAccountPasswordChangedEvent",
		"UserLoginSessionEvent":
		return ErrorEventDefinitelySkip

	// Nothing to fill here, but useful
	case
		"BadUsernameSessionEvent":
		return nil

	//esx
	case
		"esx.problem.net.vmnic.linkstate.down",
		"esx.audit.vmfs.sesparse.bloomfilter.disabled",
		"esx.clear.scsi.device.io.latency.improved",
		"esx.problem.visorfs.ramdisk.full",
		"esx.problem.vmfs.heartbeat.timedout",
		"esx.problem.storage.connectivity.lost",
		"esx.problem.storage.redundancy.degraded",
		"esx.problem.storage.redundancy.lost":
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
	case
		"com.vmware.vc.vm.VmStateRevertedToSnapshot",
		"VmPoweredOffEvent",
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
	// some strange code
	case "TaskEvent":
		if msg, err := jsonparser.GetString(jsonByte, "data", "Task"); err == nil {
			vce.Message = vce.Message + msg
		}
		vce.AlarmName, _ = jsonparser.GetString(jsonByte, "data", "Info", "DescriptionId")
		vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
		vce.DestESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "Host", "Name")
		return nil

	//Tag operations
	case "com.vmware.cis.tagging.attach":

		jsonparser.ArrayEach(jsonByte, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			key, _ := jsonparser.GetString(value, "Key")
			switch key {
			case "Tag":
				vce.Argument1, _ = jsonparser.GetString(value, "Value")
			case "Object":
				vce.EntityName, _ = jsonparser.GetString(value, "Value")
			}
		}, "data", "Arguments")
		return nil
	}

	// skip events and return only Debug record
	switch vce.Subject {
	case
		"com.vmware.vc.RestrictedAccess",
		"esx.audit.vmfs.volume.umounted",
		"com.vmware.vc.sms.EsxiVasaClientCertificateRegisterSuccess",
		"HostIsolationIpPingFailedEvent",
		"DrsSoftRuleViolationEvent",
		"NoAccessUserEvent",
		"VmRenamedEvent",
		// "BadUsernameSessionEvent",
		"CustomFieldValueChangedEvent",
		"CustomizationStartedEvent",
		"CustomizationSucceeded",
		"DatastoreFileDeletedEvent",
		"DatastoreFileNfcModificationEvent",
		"DatastoreFileUploadEvent",
		"HostSyncFailedEvent",
		"ScheduledTaskCompletedEvent",
		"ScheduledTaskStartedEvent",
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
		"DvsHostStatusUpdated",
		"HostCnxFailedNoConnectionEvent",
		"HostConnectedEvent",
		"HostConnectionLostEvent",
		"HostDisconnectedEvent",
		"HostReconnectionFailedEvent",
		"com.vmware.vcIntegrity.InstallUpdate",
		"com.vmware.vcIntegrity.Remediate",
		"com.vmware.vcIntegrity.RemediateStart",
		"com.vmware.vcIntegrity.Scan",
		"esx.audit.dcui.enabled",
		"esx.audit.net.firewall.config.changed",
		"esx.audit.net.firewall.port.hooked",
		"esx.audit.ssh.enabled",
		"esx.audit.vmfs.volume.mounted",
		"esx.clear.coredump.configured2",
		"esx.clear.net.connectivity.restored",
		"esx.clear.net.dvport.connectivity.restored",
		"esx.clear.net.dvport.redundancy.restored",
		"esx.clear.net.redundancy.restored",
		"esx.clear.net.vmnic.linkstate.up",
		"AlarmClearedEvent",
		"DVPortgroupCreatedEvent",
		"DatastoreDestroyedEvent",
		"DatastoreDiscoveredEvent",
		"DatastoreIORMReconfiguredEvent",
		"DatastoreRemovedOnHostEvent",
		"DrsResourceConfigureFailedEvent",
		"DrsResourceConfigureSyncedEvent",
		"NASDatastoreCreatedEvent",
		"VmGuestOSCrashedEvent",
		"VmInstanceUuidChangedEvent",
		"VmInstanceUuidConflictEvent",
		"VmMacConflictEvent",
		"VmRegisteredEvent",
		"VmUuidChangedEvent",
		"com.vmware.cl.SyncLibraryEvent",
		"com.vmware.license.AddLicenseEvent",
		"com.vmware.license.AssignLicenseEvent",
		"com.vmware.license.RemoveLicenseEvent",
		"com.vmware.vc.guestOperations.GuestOperation",
		"com.vmware.vcIntegrity.ImageRecommendationGenerationFinished",
		"com.vmware.vcIntegrity.NeedSyncDepots",
		"esx.audit.usb.config.changed",
		"esx.clear.storage.apd.exit",
		"esx.clear.storage.redundancy.restored",
		"esx.clear.vmfs.nfs.server.restored",
		"esx.problem.vmsyslogd.remote.failure",
		"esx.problem.vmfs.heartbeat.recovered",
		"esx.problem.clock.correction.adjtime.lostsync",
		"esx.problem.clock.correction.adjtime.sync",
		"MigrationResourceErrorEvent",
		"GeneralUserEvent":

		vce.DebugSubject = vce.Subject
		vce.Subject = ""
		return nil
	}

	// not implemented event
	// if _, err := os.Stat("test/vcenter/new/" + vce.Subject + ".json"); errors.Is(err, os.ErrNotExist) {
	// 	f, _ := os.OpenFile("test/vcenter/new/"+vce.Subject+".json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	// 	defer f.Close()
	// 	fmt.Fprintf(f, "%s", jsonByte)
	// 	fmt.Println("new event:\t" + vce.Subject)
	// }
	err = fmt.Errorf("%w", ErrorEventNotImpemented)
	return fmt.Errorf("%s %s %w", vce.Subject, jsonByte, err)
}

func (p *VCenterProcessor) HandleEvent(e *common.Event) error {

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("vcenter", "requests", "Count of all vcenter processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("vcenter", "errors", "Count of all vcenter processor errors", labels, "processor")

	if e == nil {
		errors.Inc()
		p.logger.Debug("Event is not defined")
		return nil
	}

	jsonString := e.Data.(string)
	subject, err := jsonparser.GetString([]byte(jsonString), "subject")
	if err != nil {
		errors.Inc()
		return err
	}

	chainId, err := jsonparser.GetInt([]byte(jsonString), "data", "ChainId")
	if err != nil {
		errors.Inc()
		return err
	}

	vce := vcenterEvent{
		Subject: subject,
		RAWString: strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(jsonString,
			"\\", "\\\\"),
			"\"", "\\\""),
			"'", "\\'"),
			"\n", "\\n"),
		ChainId: strconv.FormatInt(chainId, 10),
	}

	err = vce.parse([]byte(jsonString))

	if err != ErrorEventDefinitelySkip {
		if err != nil {
			errors.Inc()
			p.logger.Debug(err)
			return err
		}

		eventTime, _ := time.Parse(time.RFC3339Nano, vce.CreatedTime)
		curevent := &common.Event{
			Data:    vce,
			Channel: e.Channel,
			Type:    "VCenterEvent",
		}

		curevent.SetTime(eventTime)
		curevent.SetLogger(p.logger)
		p.outputs.Send(curevent)
	}

	return nil
}
func NewVCenterProcessor(outputs *common.Outputs, observability *common.Observability) *VCenterProcessor {

	return &VCenterProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
