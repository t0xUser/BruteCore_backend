package engine

import (
	"database/sql"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"api.brutecore/libs/lib_db"
)

type Session struct {
	id        int64
	db        *lib_db.DB
	finishing bool

	timeout int64

	proxy    *Proxy
	module   *Module
	database *Database
	workers  []*Worker
	muw      sync.Mutex

	Status struct {
		MainStatus  string
		QueuReader  bool
		ProxyReader bool
	}

	muq    sync.Mutex
	DBQueu []string
}

func NewSession(i map[string]interface{}, d *lib_db.DB) (*Session, error) {
	prox, err := NewProxy(d, i["PROXY_ID"].(int64), i["ID"].(int64))
	if err != nil {
		return nil, err
	}

	modl, err := NewModule(d, i["MODULE_ID"].(int64), i["ID"].(int64))
	if err != nil {
		return nil, err
	}

	var cdbl *Database
	if modl.mtype == "MT1" {
		cdbl, err = NewDatabase(d, i["ID"].(int64), i["DATABASE_ID"].(int64))
	} else {
		cdbl, err = NewDatabaseProtocol(d, i["ID"].(int64))
	}

	if err != nil {
		return nil, err
	}

	return &Session{
		id:       i["ID"].(int64),
		db:       d,
		timeout:  i["timeout"].(int64),
		proxy:    prox,
		module:   modl,
		database: cdbl,
	}, nil
}

func (i *Session) QueueReaderWorker() {
	for i.Status.QueuReader || len(i.DBQueu) != 0 {
		time.Sleep(time.Millisecond * 800)
		i.muq.Lock()
		tx, err := i.db.StartTx(lib_db.TxWrite)
		if err == nil {
			trueTx := tx.(*sql.Tx)
			_, err := trueTx.Exec(strings.Join(i.DBQueu, "; "))
			if err == nil {
				err := trueTx.Commit()
				if err == nil {
					i.DBQueu = nil
				} else {
					trueTx.Rollback()
				}
			}
		}
		i.muq.Unlock()
		runtime.Gosched()
	}
}

func (i *Session) StartOrCorrectWorkers(worker_count int) error {
	i.database.sWorkerC = worker_count
	if i.Status.MainStatus != SessionStatusActive {
		if err := i.fullStart(); err != nil {
			return err
		}
	}

	if len(i.workers) > worker_count {
		i.reduceWorkers(worker_count)
	} else {
		for t := len(i.workers); t < worker_count; t++ {
			worker := NewWorker(t, i)
			i.workers = append(i.workers, worker)
			go worker.execute()
		}
	}

	return nil
}

func (i *Session) fullStart() error {
	if err := i.database.GetStartBatch(i.id); err != nil {
		return err
	}

	// Запускаем воркер чтения очереди пишущих запросов к БД и Воркер Прокси
	i.Status.QueuReader = true
	go i.QueueReaderWorker()
	if i.proxy.update {
		i.Status.ProxyReader = true
		go i.proxy.ProxyWorker()
	}
	i.module.InitalizeModule()

	if _, err := i.db.Exec(lib_db.TxWrite, QStartSession, i.id, i.id); err != nil {
		return err
	}
	i.Status.MainStatus = SessionStatusActive
	return nil
}

func (i *Session) StopAndTerminate() {
	i.Status.MainStatus = SessionStatusTerminate
	i.reduceWorkers(0)
}

func (i *Session) GetActiveWorkers() int {
	return len(i.workers)
}

func (i *Session) reduceWorkers(newCount int) {
	for t := range i.workers {
		if t > newCount-1 {
			i.workers[t].work = false
		}
	}
}

/*--- CallBacks for workers ---*/
func (i *Session) setToQueue(query string) {
	i.muq.Lock()
	defer i.muq.Unlock()

	i.DBQueu = append(i.DBQueu, query)
}

func (i *Session) removeWorker(id int) {
	i.muw.Lock()
	defer i.muw.Unlock()
	indexToRemove := -1
	for i, worker := range i.workers {
		if worker.id == id {
			indexToRemove = i
			break
		}
	}

	if indexToRemove != -1 {
		i.workers = append(i.workers[:indexToRemove], i.workers[indexToRemove+1:]...)
	}

	if len(i.workers) == 0 {
		if i.finishing {
			i.setToQueue(fmt.Sprintf(QFinishSession, i.id))
			i.Status.MainStatus = SessionStatusStop
		}
		i.Status.QueuReader = false
		i.Status.ProxyReader = false
		i.proxy.Worker = false
		i.module.UnInitalizeModule()
	}
}
