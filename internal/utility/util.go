package utility

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func BoolToInt(b bool) int {
	if b {
		return 1
	} else {
		return 0
	}
}

func SplitProxy(addr string) (string, int, error) {
	if addr == "" {
		return "", 0, errors.New("Прокси не указан")
	}

	var (
		err        error
		host, port string
		portnum    int
	)

	if host, port, err = net.SplitHostPort(addr); err != nil {
		return "", 0, err
	}
	if portnum, err = net.LookupPort("tcp", port); err != nil {
		return "", 0, err
	}

	return host, portnum, nil
}

func FindFreePort() (int, error) {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

func ChangeParam(input string) string {
	parts := strings.Split(input, ",")

	var result string
	for _, part := range parts {
		result += fmt.Sprintf("'%s',", part)
	}

	return strings.TrimSuffix(result, ",")
}

func ReShape(inputType string, inputVal interface{}) interface{} {
	switch inputType {
	case "IT1", "IT3":
		return inputVal
	case "IT2":
		num, _ := strconv.ParseInt(inputVal.(string), 10, 64)
		return num
	case "IT4":
		return (inputVal.(string) == "1")
	}
	return nil
}
