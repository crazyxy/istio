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
	"fmt"
	"net"

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

	s.l = listener
	// Store the actual listening port back to the argument.
	s.Port.Port = p
	log.Infof("listening TCP on %v", p)

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
func (s *tcpInstance) echo(conn net.Conn) {
	buf := make([]byte, 1024)
	_, err := conn.Read(buf)
	if err != nil {
		log.Warn("tcp read failed: " + err.Error())
	}

	// echo the message in the buffer
	conn.Write(buf)
	conn.Close()
}

func (s *tcpInstance) Close() error {
	s.l.Close()
	return nil
}

func (s *tcpInstance) awaitReady(onReady OnReadyFunc, port int) {
	defer onReady()

	address := fmt.Sprintf("127.0.0.1:%d", port)

	err := retry.UntilSuccess(func() error {
		_, err := net.Dial("tcp", address)
		if err != nil {
			return err
		}

		// Server is up now, we're ready.
		return nil
	}, retry.Timeout(readyTimeout), retry.Delay(readyInterval))

	if err != nil {
		log.Errorf("readiness failed for endpoint %s: %v", address, err)
	} else {
		log.Infof("ready for TCP endpoint %s", address)
	}
}
