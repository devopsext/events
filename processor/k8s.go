package processor

import (
	"encoding/json"
	"errors"
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
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
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

func (p *K8sProcessor) send(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest, location string, o interface{}, patch interface{}) {

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
	if span != nil {
		e.SetSpanContext(span.GetContext())
		e.SetLogger(p.logger)
	}
	p.outputs.Send(e)
}

func (p *K8sProcessor) processNamespace(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Namespace
	var new *corev1.Namespace
	res := make(map[string]corev1.Namespace)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, name, res, patch)
}

func (p *K8sProcessor) processNode(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Node
	var new *corev1.Node
	res := make(map[string]corev1.Node)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, name, res, patch)
}

func (p *K8sProcessor) processReplicaSet(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *appsv1.ReplicaSet
	var new *appsv1.ReplicaSet
	res := make(map[string]appsv1.ReplicaSet)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processStatefulSet(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *appsv1.StatefulSet
	var new *appsv1.StatefulSet
	res := make(map[string]appsv1.StatefulSet)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processDaemonSet(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *appsv1.DaemonSet
	var new *appsv1.DaemonSet
	res := make(map[string]appsv1.DaemonSet)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processSecret(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Secret
	var new *corev1.Secret
	res := make(map[string]corev1.Secret)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processIngress(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *netv1.Ingress
	var new *netv1.Ingress
	res := make(map[string]netv1.Ingress)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processJob(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *batchv1.Job
	var new *batchv1.Job
	res := make(map[string]batchv1.Job)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processCronJob(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *batchv1beta.CronJob
	var new *batchv1beta.CronJob
	res := make(map[string]batchv1beta.CronJob)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processConfigMap(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.ConfigMap
	var new *corev1.ConfigMap
	res := make(map[string]corev1.ConfigMap)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processRole(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *rbacv1.Role
	var new *rbacv1.Role
	res := make(map[string]rbacv1.Role)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processDeployment(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *appsv1.Deployment
	var new *appsv1.Deployment
	res := make(map[string]appsv1.Deployment)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processService(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Service
	var new *corev1.Service
	res := make(map[string]corev1.Service)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processPod(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {

	var old *corev1.Pod
	var new *corev1.Pod
	res := make(map[string]corev1.Pod)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = *old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
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
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
}

func (p *K8sProcessor) processArgoApplication(span sreCommon.TracerSpan, channel string, ar *admv1beta1.AdmissionRequest) {
	var old *argov1a.Application
	var new *argov1a.Application
	res := make(map[string]*argov1a.Application)

	if ar.OldObject.Raw != nil {
		if err := json.Unmarshal(ar.OldObject.Raw, &old); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal old object: %v", err)
		}
		if old != nil {
			res["old"] = old
		}
	}

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &new); err != nil {
			p.logger.SpanError(span, "Couldn't unmarshal new object: %v", err)
		}
		if new != nil {
			res["new"] = new
		}
	}
	patch, _ := jsondiff.CompareJSON(ar.OldObject.Raw, ar.Object.Raw)
	name := new.Name
	namespace := new.Spec.Destination.Namespace
	p.send(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), res, patch)
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

	p.requests.Inc()
	p.outputs.Send(e)
	return nil
}

func (p *K8sProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	span := p.tracer.StartChildSpan(r.Header)
	defer span.Finish()

	channel := strings.TrimLeft(r.URL.Path, "/")
	p.requests.Inc()

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		p.errors.Inc()
		err := errors.New("empty body")
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	p.logger.SpanDebug(span, "Body => %s", body)

	var admissionResponse *admv1beta1.AdmissionResponse
	errorString := ""
	ar := admv1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		errorString = err.Error()
		p.logger.SpanError(span, "Can't decode body: %v", err)
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
				p.processNamespace(span, channel, req)
			case "Node":
				p.processNode(span, channel, req)
			case "ReplicaSet":
				p.processReplicaSet(span, channel, req)
			case "StatefulSet":
				p.processStatefulSet(span, channel, req)
			case "DaemonSet":
				p.processDaemonSet(span, channel, req)
			case "Secret":
				p.processSecret(span, channel, req)
			case "Ingress":
				p.processIngress(span, channel, req)
			case "Job":
				p.processJob(span, channel, req)
			case "CronJob":
				p.processCronJob(span, channel, req)
			case "ConfigMap":
				p.processConfigMap(span, channel, req)
			case "Role":
				p.processRole(span, channel, req)
			case "Deployment":
				p.processDeployment(span, channel, req)
			case "Service":
				p.processService(span, channel, req)
			case "Pod":
				p.processPod(span, channel, req)
			case "Application":
				p.processArgoApplication(span, channel, req)
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
		p.errors.Inc()
		p.logger.SpanError(span, "Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return err
	}

	if _, err := w.Write(resp); err != nil {
		p.errors.Inc()
		p.logger.SpanError(span, "Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		return err
	}

	if !utils.IsEmpty(errorString) {
		p.errors.Inc()
		err := errors.New(errorString)
		p.logger.SpanError(span, errorString)
		http.Error(w, fmt.Sprint(errorString), http.StatusInternalServerError)
		return err
	}
	return nil
}

func NewK8sProcessor(outputs *common.Outputs, observability *common.Observability) *K8sProcessor {
	return &K8sProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		tracer:  observability.Traces(),
		//	counter: observability.Metrics().Counter("requests", "Count of all k8s processor requests", []string{"user", "operation", "channel", "namespace", "kind"}, "k8s", "processor"),
		requests: observability.Metrics().Counter("k8s", "requests", "Count of all k8s processor requests", map[string]string{}, "processor", "k8s"),
		errors:   observability.Metrics().Counter("k8s", "errors", "Count of all k8s processor errors", map[string]string{}, "processor", "k8s"),
	}
}
