package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"api.brutecore/internal/utility"
	"api.brutecore/libs/lib_db"
	"github.com/shirou/gopsutil/process"
)

type Input struct {
	Type  string
	Name  string
	Value interface{}
}

type Module struct {
	id       int64
	pid      int
	mtype    string
	fileName string
	port     string
	cmd      *exec.Cmd
	mup      sync.Mutex
	inputs   *[]Input
}

type ExecuteOutput struct {
	Status string  `json:"status"`
	Log    *string `json:"log,omitempty"`
}

const (
	subp = "http://127.0.0.1:%s/ExecuteModule"
)

func NewModule(d *lib_db.DB, module_id int64, session_id int64) (*Module, error) {
	res, err := d.QueryRow(lib_db.TxRead, "SELECT PATH, TYPE FROM MODULE WHERE ID = $1", module_id)
	if err != nil {
		return nil, err
	}

	itype := (*res)[0]["type"].(string)
	fileName := (*res)[0]["path"].(string)
	if _, err := os.Stat(fileName); err != nil {
		return nil, err
	}

	res, err = d.QueryRow(lib_db.TxRead, "SELECT * FROM SESSION_DTL SD WHERE SD.SESSION_ID = $1 AND KEY IN ('IT1','IT2','IT3','IT4') AND COALESCE(VALUE, '') <> ''", session_id)
	if err != nil {
		return nil, err
	}

	var m *[]Input

	for _, v := range *res {
		if m == nil {
			m = &[]Input{}
		}
		*m = append(*m, Input{
			Type:  v["key"].(string),
			Name:  v["name"].(string),
			Value: v["value"],
		})
	}

	return &Module{
		id:       module_id,
		fileName: fileName,
		inputs:   m,
		mtype:    itype,
	}, nil
}

func (m *Module) InitalizeModule() {
	for {
		port, err := utility.FindFreePort()
		if err != nil {
			continue
		}

		m.port = strconv.Itoa(port)
		m.cmd = exec.Command(m.fileName, "-port="+m.port)
		if err := m.cmd.Start(); err != nil {
			continue
		} else {
			m.pid = m.cmd.Process.Pid
			break
		}
	}
	time.Sleep(time.Second * 3)
}

func (m *Module) UnInitalizeModule() error {
	return m.cmd.Process.Kill()
}

func (m *Module) IsRunningProccess() bool {
	_, err := process.NewProcess(int32(m.pid))
	return err == nil
}

func (m *Module) CheckRunningProccess() {
	m.mup.Lock()
	defer m.mup.Unlock()

	if !m.IsRunningProccess() {
		m.InitalizeModule()
	}
}

func (m *Module) ExecuteModule(c *ComboListRecord, p *ProxyRecord, timeout int, data_type string) (string, *string) {
	strct := make(map[string]interface{})
	strct["timeout"] = timeout

	if p != nil {
		strct["proxy_host"] = p.host
		strct["proxy_port"] = p.port
		strct["proxy_type"] = p.type_
	}

	switch data_type {
	case "DT1":
		strct["email"] = c.data
	case "DT2":
		strct["username"] = c.data
	case "DT3":
		strct["password"] = c.data
	case "DT4":
		strct["pin"] = c.data
	case "DT5", "DT6":
		parts := strings.Split(c.data, ":")
		if len(parts) == 2 {
			strct["password"] = parts[1]
			if data_type == "DT5" {
				strct["username"] = parts[0]
			} else {
				strct["email"] = parts[0]
			}
		} else {
			return "RT4", nil
		}
	case "DT7":
		strct["data"] = c.data
	case "MT2":
		strct["host"] = c.data
		strct["login"] = c.login
		strct["password"] = c.password
	default:
		return "RT4", nil
	}

	if m.inputs != nil {
		for _, v := range *m.inputs {
			strct[v.Name] = utility.ReShape(v.Type, v.Value)
		}
	}

	jsonData, err := json.Marshal(strct)
	if err != nil {
		return "RT4", nil
	}

	resp, err := http.Post(fmt.Sprintf(subp, m.port), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		m.CheckRunningProccess()
		return m.ExecuteModule(c, p, timeout, data_type)
	}
	defer resp.Body.Close()

	var outputData ExecuteOutput
	if err := json.NewDecoder(resp.Body).Decode(&outputData); err != nil {
		return "RT4", nil
	}

	return outputData.Status, outputData.Log
}
