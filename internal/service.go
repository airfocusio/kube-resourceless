package internal

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	admission "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecFactory  = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecFactory.UniversalDeserializer()
)

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admission.AddToScheme(runtimeScheme)
	_ = appsv1.AddToScheme(runtimeScheme)
}

type admitv1Func func(admission.AdmissionReview) *admission.AdmissionResponse

type admitHandler struct {
	v1 admitv1Func
}

func AdmitHandler(f admitv1Func) admitHandler {
	return admitHandler{
		v1: f,
	}
}

type ServiceOpts struct {
	TLSCertFile string
	TLSKeyFile  string
}

type Service struct {
	Opts      ServiceOpts
	Ctx       context.Context
	Config    *rest.Config
	Clientset *kubernetes.Clientset
}

func NewService(opts ServiceOpts) (*Service, error) {
	service := &Service{Opts: opts}
	service.Ctx = context.Background()

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to create kubernetes rest config: %w", err)
	}
	service.Config = config

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create kubernetes rest clientset: %w", err)
	}
	service.Clientset = clientset

	return service, nil
}

func (s *Service) Run(stop <-chan os.Signal) error {
	Info.Printf("starting server...\n")

	certLoader := CertLoader{
		CertFile: s.Opts.TLSCertFile,
		KeyFile:  s.Opts.TLSKeyFile,
	}
	tlsConfig := &tls.Config{
		GetCertificate: certLoader.GetCertificate,
	}
	tlsListener, err := tls.Listen("tcp", ":8443", tlsConfig)
	if err != nil {
		Error.Printf("listening failed: %v\n", err)
		return err
	}

	http.HandleFunc("/mutate", func(w http.ResponseWriter, r *http.Request) {
		s.ServeAdmitHandler(w, r, AdmitHandler(s.Mutate))
	})

	if err := http.Serve(tlsListener, nil); err != nil {
		Error.Printf("starting server failed: %v\n", err)
		return err
	}

	return nil
}

func (s *Service) Mutate(ar admission.AdmissionReview) *admission.AdmissionResponse {
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		Error.Printf("expect resource to be %s\n", podResource)
		return &admission.AdmissionResponse{
			Result: &metav1.Status{
				Message: fmt.Sprintf("expect resource to be %s\n", podResource),
			},
		}
	}

	Info.Printf("mutating pod")

	raw := ar.Request.Object.Raw
	pod := corev1.Pod{}

	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		Error.Printf("deserialization failed: %v\n", err)
		return &admission.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	patchType := admission.PatchTypeJSONPatch
	patchOps := []string{}
	for i := range pod.Spec.Containers {
		patchOps = append(patchOps, fmt.Sprintf(`{"op":"remove","path":"/spec/containers/%d/resources"}`, i))
	}
	for i := range pod.Spec.InitContainers {
		patchOps = append(patchOps, fmt.Sprintf(`{"op":"remove","path":"/spec/initContainers/%d/resources"}`, i))
	}
	patch := fmt.Sprintf("[%s]", strings.Join(patchOps, ","))
	return &admission.AdmissionResponse{Allowed: true, PatchType: &patchType, Patch: []byte(patch)}
}

// serve handles the http portion of a request prior to handing to an admit function
func (s *Service) ServeAdmitHandler(w http.ResponseWriter, r *http.Request, admit admitHandler) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, fmt.Sprintf("expect content type application/json but got %s", contentType), http.StatusBadRequest)
		return
	}

	Debug.Printf("handling request: %s", body)
	var responseObj runtime.Object
	if obj, gvk, err := deserializer.Decode(body, nil, nil); err != nil {
		Error.Printf("decoding request failed: %v", err)
		http.Error(w, "decoding request failed", http.StatusBadRequest)
		return

	} else {
		requestedAdmissionReview, ok := obj.(*admission.AdmissionReview)
		if !ok {
			http.Error(w, fmt.Sprintf("expected v1.AdmissionReview but got %T", obj), http.StatusBadRequest)
			return
		}
		responseAdmissionReview := &admission.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = admit.v1(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview

	}
	Debug.Printf("sending response: %v\n", responseObj)
	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		Error.Printf("marshalling response failed: %v\n", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		Error.Printf("writing response failed: %v\n", err)
	}
}
