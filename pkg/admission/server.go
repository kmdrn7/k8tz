/*
Copyright © 2021 Yonatan Kahana

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admission

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/k8tz/k8tz/pkg/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	jsonContentType = `application/json`
)

var (
	k8sdecode       = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	podResource     = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
	cronJobResource = metav1.GroupVersionResource{Version: "v1", Resource: "cronjobs", Group: "batch"}
	verboseLogger   *log.Logger
	warningLogger   *log.Logger
	infoLogger      *log.Logger
	errorLogger     *log.Logger
)

type Server struct {
	TLSCertFile string
	TLSKeyFile  string
	Address     string
	Handler     RequestsHandler
	Verbose     bool
}

func NewAdmissionServer() *Server {
	return &Server{
		TLSCertFile: "/run/secrets/tls/tls.crt",
		TLSKeyFile:  "/run/secrets/tls/tls.key",
		Address:     ":8443",
		Handler:     NewRequestsHandler(),
		Verbose:     false,
	}
}

func (h *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Server) Start(kubeconfigFlag string) error {
	infoLogger.Println(version.DisplayVersion())

	if h.Verbose {
		verboseLogger.SetOutput(os.Stderr)
		verboseLogger.Printf("server=%+v", *h)
	}

	err := h.Handler.InitializeClientset(kubeconfigFlag)
	if err != nil {
		return fmt.Errorf("failed to setup connection with kubernetes api: %w", err)
	}

	infoLogger.Printf("Listening on %s\n", h.Address)

	mux := http.NewServeMux()

	mux.HandleFunc("/", h.Handler.handleFunc)
	mux.HandleFunc("/health", h.health)

	server := &http.Server{
		Addr:    h.Address,
		Handler: mux,
		TLSConfig: &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.LoadX509KeyPair(h.TLSCertFile, h.TLSKeyFile)
				if err != nil {
					return nil, err
				}
				return &cert, nil
			},
			CipherSuites: []uint16{
				// TLSv1.0 & TLSv1.1
				tls.TLS_RSA_WITH_AES_128_CBC_SHA, //nolint:gosec
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				// TLSv1.2
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
				// TLSv1.3
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
		},
	}

	return server.ListenAndServeTLS("", "")
}

func init() {
	verboseLogger = log.New(io.Discard, "VERBOSE: ", log.Ldate|log.Ltime|log.Lshortfile)
	infoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger = log.New(os.Stderr, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}
