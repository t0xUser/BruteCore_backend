package engine

import (
	"log"
	"strconv"
	"sync"
	"time"

	"api.brutecore/libs/lib_db"
)

type Pulling struct {
	db           *lib_db.DB
	timeOut      time.Duration
	work         bool
	mup          sync.Mutex
	InstancePool []*Session
}

const (
	SessionStatusActive    = "ST5"
	SessionStatusStop      = "ST6"
	SessionStatusTerminate = "ST7"
)

func NewPulling(d *lib_db.DB, t time.Duration) *Pulling {
	return &Pulling{
		db:      d,
		timeOut: t,
		work:    false,
	}
}

func (p *Pulling) StartListen() {
	p.work = true
	log.Println("Engine initalized")
	for p.work {
		p.Iteration()
		time.Sleep(p.timeOut)
	}
}

func (p *Pulling) StopListen() {
	p.work = false
}

func (p *Pulling) getWorkingIDs() string {
	str := "("
	for _, i := range p.InstancePool {
		str = str + strconv.FormatInt(i.id, 10) + ", "
	}
	return str + "-1)"
}

func (p *Pulling) GetProxyCount(id int64) int {
	i := p.getInstanceByID(id)
	if i == nil {
		return 0
	}
	return len(i.proxy.buffer)
}

func (p *Pulling) getInstanceByID(id int64) *Session {
	for _, instance := range p.InstancePool {
		if instance.id == id {
			return instance
		}
	}
	return nil
}

func (p *Pulling) removeInstance(instanceToRemove *Session) {
	for i, instance := range p.InstancePool {
		if instance == instanceToRemove {
			p.InstancePool = append(p.InstancePool[:i], p.InstancePool[i+1:]...)
			return
		}
	}
}

func (p *Pulling) writeError(err error, id int64) {
	p.db.Exec(lib_db.TxWrite, QSetSessionError, err.Error(), id)
}

func (p *Pulling) Iteration() {
	p.mup.Lock() //ЭТО РАБОТАЕТ, НЕ ТРОГАТЬ!! Иначе Iteration вызывается циктом и из эндпоинта, двойной вызов и начальные строки начинают двоиться, просто не трогай
	defer p.mup.Unlock()
	res, err := p.db.QueryRow(lib_db.TxRead, QGetAllAliveSessions+p.getWorkingIDs())
	if err != nil {
		return
	}

	for _, i := range *res {
		var inst *Session
		ID := i["ID"].(int64)

		if inst = p.getInstanceByID(ID); inst == nil {
			if inst, err = NewInstance(i, p.db); err != nil {
				p.writeError(err, ID)
				continue
			}
			p.InstancePool = append(p.InstancePool, inst)
		}

		WC := int(i["W"].(int64))
		if i["S"].(string) != inst.Status.MainStatus || WC != inst.GetActiveWorkers() {
			switch i["S"].(string) {
			case SessionStatusActive:
				if !inst.finishing {
					if err := inst.StartOrCorrectWorkers(WC); err != nil {
						p.writeError(err, ID)
					}
				}
			case SessionStatusStop:
				p.removeInstance(inst)
			case SessionStatusTerminate:
				if inst.GetActiveWorkers() == 0 {
					p.removeInstance(inst)
				} else {
					inst.StopAndTerminate()
				}
			}
		}
	}
}
