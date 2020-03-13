package processor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/devopsext/events/common"
	"github.com/prometheus/client_golang/prometheus"
	admv1beta1 "k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimek8s "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var k8sProcessorRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_k8s_processor_requests",
	Help: "Count of all k8s processor requests",
}, []string{"k8s_processor_user", "k8s_processor_operation", "k8s_processor_orchestration", "k8s_processor_namespace", "k8s_processor_kind"})

type K8sProcessor struct {
	outputs *common.Outputs
}

var (
	runtimeScheme = runtimek8s.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

func (p *K8sProcessor) prepareOperation(operation admv1beta1.Operation) string {

	return strings.Title(strings.ToLower(string(operation)))
}

func (p *K8sProcessor) processK8sNamespace(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var namespace *corev1.Namespace

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &namespace); err != nil {

			log.Error("Couldn't unmarshal namespace object: %v", err)
		}
	}

	name := ar.Name

	if namespace != nil {

		name = namespace.Name
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      name,
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        namespace,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sNode(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var node *corev1.Node

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &node); err != nil {

			log.Error("Couldn't unmarshal node object: %v", err)
		}
	}

	name := ar.Name

	if node != nil {

		name = node.Name
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      name,
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        node,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sReplicaSet(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var replicaSet *appsv1.ReplicaSet

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &replicaSet); err != nil {

			log.Error("Couldn't unmarshal replicaSet object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if replicaSet != nil {

		name = replicaSet.Name
		namespace = replicaSet.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        replicaSet,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sStatefulSet(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var statefulSet *appsv1.StatefulSet

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &statefulSet); err != nil {

			log.Error("Couldn't unmarshal statefulSet object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if statefulSet != nil {

		name = statefulSet.Name
		namespace = statefulSet.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        statefulSet,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sDaemonSet(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var daemonSet *appsv1.DaemonSet

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &daemonSet); err != nil {

			log.Error("Couldn't unmarshal daemonSet object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if daemonSet != nil {

		name = daemonSet.Name
		namespace = daemonSet.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        daemonSet,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sSecret(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var secret *corev1.Secret

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &secret); err != nil {

			log.Error("Couldn't unmarshal secret object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if secret != nil {

		name = secret.Name
		namespace = secret.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        secret,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sIngress(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var ingress *netv1beta1.Ingress

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &ingress); err != nil {

			log.Error("Couldn't unmarshal ingress object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if ingress != nil {

		name = ingress.Name
		namespace = ingress.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        ingress,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sJob(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var job *batchv1.Job

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &job); err != nil {

			log.Error("Couldn't unmarshal job object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if job != nil {

		name = job.Name
		namespace = job.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        job,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sCronJob(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var cronJob *batchv1beta.CronJob

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &cronJob); err != nil {

			log.Error("Couldn't unmarshal cronjob object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if cronJob != nil {

		name = cronJob.Name
		namespace = cronJob.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        cronJob,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sConfigMap(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var configMap *corev1.ConfigMap

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &configMap); err != nil {

			log.Error("Couldn't unmarshal configMap object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if configMap != nil {

		name = configMap.Name
		namespace = configMap.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        configMap,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sRole(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var role *rbacv1.Role

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &role); err != nil {

			log.Error("Couldn't unmarshal role object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if role != nil {

		name = role.Name
		namespace = role.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        role,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sDeployment(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var deployment *appsv1.Deployment

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &deployment); err != nil {

			log.Error("Couldn't unmarshal deployment object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if deployment != nil {

		name = deployment.Name
		namespace = deployment.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        deployment,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sService(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var service *corev1.Service

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &service); err != nil {

			log.Error("Couldn't unmarshal service object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if service != nil {

		name = service.Name
		namespace = service.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        service,
		User:          user,
	})
}

func (p *K8sProcessor) processK8sPod(orchestration string, user *common.User, ar *admv1beta1.AdmissionRequest) {

	var pod *corev1.Pod

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &pod); err != nil {

			log.Error("Couldn't unmarshal pod object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if pod != nil {

		name = pod.Name
		namespace = pod.Namespace
	}

	p.outputs.Send(&common.Event{
		Orchestration: orchestration,
		Location:      fmt.Sprintf("%s.%s", namespace, name),
		Kind:          ar.Kind.Kind,
		Operation:     p.prepareOperation(ar.Operation),
		Object:        pod,
		User:          user,
	})
}

func (p *K8sProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {

	var body []byte

	if r.Body != nil {

		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {

		log.Error("Empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	log.Debug("Body => %s", body)

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {

		log.Error("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect application/json", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admv1beta1.AdmissionResponse
	ar := admv1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {

		log.Error("Can't decode body: %v", err)

		admissionResponse = &admv1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {

		req := ar.Request
		orchestration := strings.TrimLeft(r.URL.Path, "/")
		user := &common.User{Name: req.UserInfo.Username, ID: req.UserInfo.UID}

		switch req.Kind.Kind {
		case "Namespace":
			p.processK8sNamespace(orchestration, user, req)
		case "Node":
			p.processK8sNode(orchestration, user, req)
		case "ReplicaSet":
			p.processK8sReplicaSet(orchestration, user, req)
		case "StatefulSet":
			p.processK8sStatefulSet(orchestration, user, req)
		case "DaemonSet":
			p.processK8sDaemonSet(orchestration, user, req)
		case "Secret":
			p.processK8sSecret(orchestration, user, req)
		case "Ingress":
			p.processK8sIngress(orchestration, user, req)
		case "Job":
			p.processK8sJob(orchestration, user, req)
		case "CronJob":
			p.processK8sCronJob(orchestration, user, req)
		case "ConfigMap":
			p.processK8sConfigMap(orchestration, user, req)
		case "Role":
			p.processK8sRole(orchestration, user, req)
		case "Deployment":
			p.processK8sDeployment(orchestration, user, req)
		case "Service":
			p.processK8sService(orchestration, user, req)
		case "Pod":
			p.processK8sPod(orchestration, user, req)
		}

		k8sProcessorRequests.WithLabelValues(req.UserInfo.Username, string(req.Operation), orchestration, req.Namespace, req.Kind.Kind).Inc()

		admissionResponse = &admv1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	admissionReview := admv1beta1.AdmissionReview{}
	if admissionResponse != nil {

		admissionReview.Response = admissionResponse
		if ar.Request != nil {

			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {

		log.Error("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := w.Write(resp); err != nil {

		log.Error("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func NewK8sProcessor(outputs *common.Outputs) *K8sProcessor {
	return &K8sProcessor{outputs: outputs}
}

func init() {
	prometheus.Register(k8sProcessorRequests)
}
