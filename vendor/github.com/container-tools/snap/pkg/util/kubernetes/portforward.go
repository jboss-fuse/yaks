package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func PortForward(ctx context.Context, config *restclient.Config, ns, pod string, stdOut, stdErr io.Writer) (host string, err error) {
	client, err := corev1client.NewForConfig(config)
	if err != nil {
		return "", err
	}

	url := client.RESTClient().Post().
		Resource("pods").
		Namespace(ns).
		Name(pod).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return "", err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)
	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	forwarder, err := portforward.New(dialer, []string{":9000"}, stopChan, readyChan, stdOut, stdErr)
	if err != nil {
		return "", err
	}

	go func() {
		// Start the port forwarder
		err = forwarder.ForwardPorts()
		if err != nil {
			log.Print("Error while forwarding ports: ", err)
		}
	}()

	go func() {
		// Stop the port forwarder when the context ends
		select {
		case <-ctx.Done():
			close(stopChan)
		}
	}()

	select {
	case <-readyChan:
		ports, err := forwarder.GetPorts()
		if err != nil {
			return "", err
		}
		if len(ports) != 1 {
			return "", errors.New("wrong ports opened")
		}
		return fmt.Sprintf("localhost:%d", ports[0].Local), nil
	case <-ctx.Done():
		return "", errors.New("context closed")
	}
}
