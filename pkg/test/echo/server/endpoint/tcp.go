// Copyright 2020 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package endpoint

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"istio.io/istio/pkg/test/echo/common/response"

	"istio.io/istio/pkg/test/util/retry"
	"istio.io/pkg/log"
)

var _ Instance = &tcpInstance{}

type tcpInstance struct {
	Config
	l net.Listener
}

func newTCP(config Config) Instance {
	return &tcpInstance{
		Config: config,
	}
}

func (s *tcpInstance) Start(onReady OnReadyFunc) error {
	// Listen on the given port and update the port if it changed from what was passed in.
	listener, p, err := listenOnPort(s.Port.Port)
	if err != nil {
		return err
	}

	// Store the actual listening port back to the argument.
	s.Port.Port = p
	s.l = listener

	fmt.Printf("Listening TCP on %v\n", p)

	// Start serving TCP traffic.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Warn("tcp accept failed: " + err.Error())
				return
			}

			go s.echo(conn)
		}
	}()

	// Notify the WaitGroup once the port has transitioned to ready.
	go s.awaitReady(onReady, p)

	return nil
}

// Handles incoming connection.
func (s *tcpInstance) echo(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()

	// Fill the field in the response
	_, _ = conn.Write([]byte(fmt.Sprintf("%s=%s\n", string(response.StatusCodeField), response.StatusCodeOK)))

	buf := make([]byte, 2048)
	n, err := bufio.NewReader(conn).Read(buf)
	if err != nil {
		return
	}

	// echo the message in the buffer
	_, _ = conn.Write(buf[:n])
}

func (s *tcpInstance) Close() error {
	if s.l != nil {
		s.l.Close()
	}
	return nil
}

func (s *tcpInstance) awaitReady(onReady OnReadyFunc, port int) {
	defer onReady()

	address := fmt.Sprintf("127.0.0.1:%d", port)

	err := retry.UntilSuccess(func() error {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return err
		}
		defer conn.Close()

		// Server is up now, we're ready.
		return nil
	}, retry.Timeout(readyTimeout), retry.Delay(readyInterval))

	if err != nil {
		log.Errorf("readiness failed for endpoint %s: %v", address, err)
	} else {
		log.Infof("ready for TCP endpoint %s", address)
	}
}
