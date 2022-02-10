package tests

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type MockServer struct {
	host     string
	port     int
	listener net.Listener
	once     sync.Once
	expected map[Command]Expectation
}

type Expectation struct {
	times       int
	commandType CommandType
}

type isExpectedFunc func(command Command) (bool, CommandType)

type TcpClient struct {
	conn       net.Conn
	isExpected isExpectedFunc
}

type CommandType string
type Command string

const (
	RETURN_FAIL  CommandType = "FAIL"
	RETURN_VIRUS CommandType = "VIRUS"
	RETURN_OK    CommandType = "OK"

	INSTREAM = "zINSTREAM"
	PING     = "PING"
	STATS    = "zSTATS"
	RELOAD   = "RELOAD"
)

func NewMockServer(host string, port int) *MockServer {
	return &MockServer{
		host:     host,
		port:     port,
		expected: make(map[Command]Expectation),
	}
}

func (server *MockServer) GetExpected() map[Command]Expectation {
	return server.expected
}

func (server *MockServer) Expect(command Command, times int, commandType CommandType) {
	server.expected[command] = struct {
		times       int
		commandType CommandType
	}{times, commandType}
}

func (server *MockServer) isExpected(command Command) (bool, CommandType) {
	if c, found := server.expected[command]; found {
		if c.times > 0 {
			c.times--
			return true, c.commandType
		}
	}
	return false, ""
}

func (server *MockServer) Shutdown() {
	if server.listener != nil {
		server.once.Do(func() { server.listener.Close() })
	}
}

func (server *MockServer) Run() {
	var err error
	server.listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", server.host, server.port))
	if err != nil {
		panic(err)
	}
	defer server.Shutdown()
	for {
		conn, err := server.listener.Accept()
		if err != nil {
			panic(err)
		}

		fmt.Printf("Connection accepted: %s\n", conn.RemoteAddr().String())

		client := &TcpClient{
			conn:       conn,
			isExpected: server.isExpected,
		}
		go client.handleRequest()
	}
}

func (client *TcpClient) shutdownWrite() {
	if v, ok := client.conn.(interface{ CloseWrite() error }); ok {
		v.CloseWrite()
	}
}

func (client *TcpClient) expectedOrDie(command Command) CommandType {
	expected, commandType := client.isExpected(command)
	if !expected {
		panic("unexpected \"" + string(command) + "\" command")
	}
	return commandType
}

func (client *TcpClient) handleRequest() {
	reader := bufio.NewReader(client.conn)
	writer := client.conn.(io.Writer)
	defer client.conn.Close()

	command, err := reader.ReadString('\000')
	if err != nil {
		panic(err)
	}

	command = strings.Trim(command, "\000")

	var resp []byte
	fmt.Printf("Received command: \"%s\"\n", command)
	switch command {
	case INSTREAM:
		time.Sleep(1 * time.Second)

		commandType := client.expectedOrDie(Command(command))
		if commandType == RETURN_FAIL {
			return // close connection
		} else if commandType == RETURN_OK {
			resp = []byte("Stream: OK\n")
		} else {
			resp = []byte("Stream: Virus\n")
		}
		_, err := writer.Write(resp)
		if err != nil {
			panic(err)
		}

	case PING:
		time.Sleep(1 * time.Second)

		commandType := client.expectedOrDie(Command(command))
		if commandType == RETURN_FAIL {
			return // close connection
		} else if commandType == RETURN_OK {
			resp = []byte("PONG\n")
		} else {
			resp = []byte("FOOBAR\n")
		}
		_, err := writer.Write(resp)
		if err != nil {
			panic(err)
		}

	case STATS:
		time.Sleep(1 * time.Second)

		commandType := client.expectedOrDie(Command(command))
		if commandType == RETURN_FAIL {
			return // close connection
		} else if commandType == RETURN_OK {
			resp = []byte("PONG\n")
		} else {
			resp = []byte("FOOBAR\n")
		}
		_, err := writer.Write(resp)
		if err != nil {
			panic(err)
		}

	case RELOAD:
		time.Sleep(1 * time.Second)

		commandType := client.expectedOrDie(Command(command))
		if commandType == RETURN_FAIL {
			return // close connection
		} else if commandType == RETURN_OK {
			resp = []byte("PONG\n")
		} else {
			resp = []byte("FOOBAR\n")
		}
		_, err := writer.Write(resp)
		if err != nil {
			panic(err)
		}

	default:
		panic("UNKNOWN COMMAND")

	}
	client.shutdownWrite()
}
