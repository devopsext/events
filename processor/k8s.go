package processor

import (
	"encoding/json"
	errPkg "errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/serializer"

	argov1a "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	jsondiff "github.com/wI2L/jsondiff"
	admv1beta1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimek8s "k8s.io/apimachinery/pkg/runtime"
)

type K8sProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
}

type K8sUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type K8sData struct {
	Kind      string      `json:"kind"`
	Location  string      `json:"location"`
	Operation string      `json:"operation"`
	Namespace string      `json:"namespace"`
	Object    interface{} `json:"object,omitempty"`
	Patch     interface{} `json:"patch,omitempty"`
	User      *K8sUser    `json:"user"`
}

var (
	runtimeScheme = runtimek8s.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

func K8sProcessorType() string {
	return "K8s"
}

func (p *K8sProcessor) EventType() string {
	return common.AsEventType(K8sProcessorType())
}

func (p *K8sProcessor) prepareOperation(operation admv1beta1.Operation) string {
	return strings.Title(strings.ToLower(string(operation)))
}

func (p *K8sProcessor) send(channel string, ar *admv1beta1.AdmissionRequest, location string, o interface{}, patch interface{}) {

	user := &K8sUser{Name: ar.UserInfo.Username, ID: ar.UserInfo.UID}
	operation := p.prepareOperation(ar.Operation)

	e := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data: K8sData{
			Kind:      ar.Kind.Kind,
			Operation: operation,
			Namespace: ar.Namespace,
			Location:  location,
			Object:    o,
			Patch:     patch,
			User:      user,
		},
	}
	e.SetTime(time.Now().UTC())
	e.SetLogger(p.logger)
	p.outputs.Send(e)
}

func (p *K8sProcessor) processNamespace(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Namespace
	var new *corev1.Namespace
	res := make(map[string]corev1.Namespace)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	if len(res) > 0 {
		name = new.Name
	}
	p.send(channel, ar, name, res, patch)
}

func (p *K8sProcessor) processNode(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Node
	var new *corev1.Node
	res := make(map[string]corev1.Node)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	if len(res) > 0 {
		name = new.Name
	}
	p.send(channel, ar, name, res, patch)
}

func (p *K8sProcessor) processReplicaSet(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *appsv1.ReplicaSet
	var new *appsv1.ReplicaSet
	res := make(map[string]appsv1.ReplicaSet)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processStatefulSet(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *appsv1.StatefulSet
	var new *appsv1.StatefulSet
	res := make(map[string]appsv1.StatefulSet)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processDaemonSet(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *appsv1.DaemonSet
	var new *appsv1.DaemonSet
	res := make(map[string]appsv1.DaemonSet)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processSecret(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Secret
	var new *corev1.Secret
	res := make(map[string]corev1.Secret)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processIngress(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *netv1.Ingress
	var new *netv1.Ingress
	res := make(map[string]netv1.Ingress)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processJob(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *batchv1.Job
	var new *batchv1.Job
	res := make(map[string]batchv1.Job)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processCronJob(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *batchv1beta.CronJob
	var new *batchv1beta.CronJob
	res := make(map[string]batchv1beta.CronJob)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processConfigMap(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.ConfigMap
	var new *corev1.ConfigMap
	res := make(map[string]corev1.ConfigMap)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processRole(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *rbacv1.Role
	var new *rbacv1.Role
	res := make(map[string]rbacv1.Role)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processDeployment(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *appsv1.Deployment
	var new *appsv1.Deployment
	res := make(map[string]appsv1.Deployment)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processService(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Service
	var new *corev1.Service
	res := make(map[string]corev1.Service)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processPod(channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Pod
	var new *corev1.Pod
	res := make(map[string]corev1.Pod)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = *new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := ar.Name
	namespace := ar.Namespace
	if len(res) > 0 {
		name = new.Name
		namespace = new.Namespace
	}
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processArgoApplication(channel string, ar *admv1beta1.AdmissionRequest) {
	var old *argov1a.Application
	var new *argov1a.Application
	res := make(map[string]*argov1a.Application)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.Error("Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.Error("Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := new.Name
	namespace := new.Spec.Destination.Namespace
	p.send(channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	/*json, err := e.JsonBytes()
	if err != nil {
		p.logger.Error(err)
		return
	}

	userName := gjson.GetBytes(json, "data.user.name").String()
	operation := gjson.GetBytes(json, "data.operation").String()
	namespace := gjson.GetBytes(json, "data.namespace").String()
	kind := gjson.GetBytes(json, "data.kind").String()
	p.counter.Inc(userName, operation, e.Channel, namespace, kind)
	*/

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("k8s", "requests", "Count of all k8s processor requests", labels, "processor")
	requests.Inc()

	p.outputs.Send(e)
	return nil
}

func (p *K8sProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("k8s", "requests", "Count of all k8s processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("k8s", "errors", "Count of all k8s processor errors", labels, "processor")

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		errors.Inc()
		err := errPkg.New("empty body")
		p.logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	p.logger.Debug("Body => %s", body)

	var admissionResponse *admv1beta1.AdmissionResponse
	errorString := ""
	ar := admv1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		errorString = err.Error()
		p.logger.Error("Can't decode body: %v", err)
		admissionResponse = &admv1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: errorString,
			},
		}
	} else {

		req := ar.Request

		// Do not process DryRun requests - we only want real changes, also removes event duplicates (dryRun goes first)
		if !*req.DryRun {
			switch req.Kind.Kind {
			case "Namespace":
				p.processNamespace(channel, req)
			case "Node":
				p.processNode(channel, req)
			case "ReplicaSet":
				p.processReplicaSet(channel, req)
			case "StatefulSet":
				p.processStatefulSet(channel, req)
			case "DaemonSet":
				p.processDaemonSet(channel, req)
			case "Secret":
				p.processSecret(channel, req)
			case "Ingress":
				p.processIngress(channel, req)
			case "Job":
				p.processJob(channel, req)
			case "CronJob":
				p.processCronJob(channel, req)
			case "ConfigMap":
				p.processConfigMap(channel, req)
			case "Role":
				p.processRole(channel, req)
			case "Deployment":
				p.processDeployment(channel, req)
			case "Service":
				p.processService(channel, req)
			case "Pod":
				p.processPod(channel, req)
			case "Application":
				p.processArgoApplication(channel, req)
			}
		}

		admissionResponse = &admv1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	admissionReview := admv1beta1.AdmissionReview{}
	admissionReview.APIVersion = ar.APIVersion
	admissionReview.Kind = ar.Kind

	admissionReview.Response = admissionResponse
	if ar.Request != nil {
		admissionReview.Response.UID = ar.Request.UID
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		errors.Inc()
		p.logger.Error("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return err
	}

	if _, err := w.Write(resp); err != nil {
		errors.Inc()
		p.logger.Error("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		return err
	}

	if !utils.IsEmpty(errorString) {
		errors.Inc()
		err := errPkg.New(errorString)
		p.logger.Error(errorString)
		http.Error(w, fmt.Sprint(errorString), http.StatusInternalServerError)
		return err
	}
	return nil
}

func NewK8sProcessor(outputs *common.Outputs, observability *common.Observability) *K8sProcessor {
	return &K8sProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
