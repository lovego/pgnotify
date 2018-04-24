package pgnotify

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/lovego/errs"
	"github.com/lovego/logger"
)

type Notifier struct {
	dbAddr   string
	db       *sql.DB
	listener *pq.Listener
	logger   *logger.Logger
	handlers map[string]Handler
}

type Handler interface {
	//	Reload(table string, buf []byte)
	Create(table string, buf []byte)
	Update(table string, buf []byte)
	Delete(table string, buf []byte)
}

type Message struct {
	Action string
	Data   json.RawMessage
}

func New(dbAddr string, logger *logger.Logger) (*Notifier, error) {
	db, err := sql.Open(`postgres`, dbAddr)
	if err != nil {
		return nil, errs.Trace(err)
	}
	if err := db.Ping(); err != nil {
		return nil, errs.Trace(err)
	}
	db.SetConnMaxLifetime(time.Minute)
	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)

	if err := CreateFunction(db); err != nil {
		return nil, err
	}
	return &Notifier{
		dbAddr: dbAddr, db: db,
		logger: logger, handlers: make(map[string]Handler),
	}, nil
}

func (n *Notifier) Notify(table string, handler Handler) error {
	if err := CreateTriggerIfNotExists(n.db, table); err != nil {
		return err
	}
	if n.listener == nil {
		n.listener = pq.NewListener(n.dbAddr, time.Nanosecond, time.Minute, n.eventLogger)
		go n.listen()
	}
	n.handlers[table] = handler
	if err := n.listener.Listen("pgnotify_" + table); err != nil {
		return errs.Trace(err)
	}
	return nil
}

func (n *Notifier) listen() {
	for {
		select {
		case notice := <-n.listener.Notify:
			n.handle(notice)
		case <-time.After(time.Minute):
			go n.listener.Ping()
		}
	}
}

func (n *Notifier) handle(notice *pq.Notification) {
	if notice == nil {
		// connection loss
		return
	}

	var table = strings.TrimPrefix(notice.Channel, "pgnotify_")
	handler := n.handlers[table]
	if handler == nil {
		n.logger.Errorf("unexpected notification: %+v", notice)
	}

	var msg Message
	if err := json.Unmarshal([]byte(notice.Extra), &msg); err != nil {
		n.logger.Error(err)
	}
	switch msg.Action {
	case "INSERT":
		handler.Create(table, msg.Data)
	case "UPDATE":
		handler.Update(table, msg.Data)
	case "DELETE":
		handler.Delete(table, msg.Data)
	default:
		n.logger.Errorf("unexpected msg: %+v", msg)
	}
}

func (n *Notifier) eventLogger(event pq.ListenerEventType, err error) {
	if err != nil {
		n.logger.Error(event, err)
	}
}
