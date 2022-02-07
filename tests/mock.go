package tests

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

type MockServer struct {
	host     string
	port     string
	expected map[Command]struct {
		times       int
		commandType CommandType
	}
}

type isExpectedFunc func(command Command) (bool, CommandType)

type TcpClient struct {
	conn       net.Conn
	isExpected isExpectedFunc
}

type CommandType string
type Command string

const (
	FAIL  CommandType = "FAIL"
	VIRUS CommandType = "VIRUS"
	OK    CommandType = "OK"

	Z_INSTREAM = "zINSTREAM"
)

func NewMockServer(host, port string) *MockServer {
	return &MockServer{
		host, port, make(map[Command]struct {
			times       int
			commandType CommandType
		}),
	}
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

func (server *MockServer) Run() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", server.host, server.port))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
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

func (client *TcpClient) handleRequest() {
	reader := bufio.NewReader(client.conn)
	defer client.conn.Close()
	writer := client.conn.(io.Writer)
	command, err := reader.ReadString('\000')

	if err != nil {
		panic(err)
	}

	switch strings.Trim(command, "\000") {

	case "zINSTREAM":
		var resp []byte

		fmt.Println("zINSTREAM COMMAND")
		reader.ReadString('\000')
		time.Sleep(1 * time.Second)

		expected, commandType := client.isExpected(Z_INSTREAM)

		if !expected {
			panic("unexpected zINSTREAM command")
		}

		if commandType == FAIL {
			return // close connection

		} else if commandType == OK {
			resp = []byte("Stream: OK\n")

		} else {
			resp = []byte("Stream: Virus\n")
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
