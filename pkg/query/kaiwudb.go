package query

import (
	"fmt"
	"sync"
)

type Kaiwudb struct {
	id               uint64
	HumanLabel       []byte
	HumanDescription []byte
	Hypertable       []byte
	SqlQuery         []byte
}

var KaiwudbPool = sync.Pool{
	New: func() interface{} {
		return &Kaiwudb{
			HumanLabel:       make([]byte, 0, 1024),
			HumanDescription: make([]byte, 0, 1024),
			Hypertable:       make([]byte, 0, 1024),
			SqlQuery:         make([]byte, 0, 1024),
		}
	},
}

func NewKaiwudb() *Kaiwudb {
	return KaiwudbPool.Get().(*Kaiwudb)
}

func (q *Kaiwudb) Release() {
	q.HumanLabel = q.HumanLabel[:0]
	q.HumanDescription = q.HumanDescription[:0]
	q.id = 0

	q.Hypertable = q.Hypertable[:0]
	q.SqlQuery = q.SqlQuery[:0]
	KaiwudbPool.Put(q)
}

func (q *Kaiwudb) HumanLabelName() []byte {
	return q.HumanLabel
}

func (q *Kaiwudb) HumanDescriptionName() []byte {
	return q.HumanDescription
}

func (q *Kaiwudb) GetID() uint64 {
	return q.id
}

func (q *Kaiwudb) SetID(n uint64) {
	q.id = n
}

func (q *Kaiwudb) String() string {
	return fmt.Sprintf("HumanLabel: %s, HumanDescription: %s, Hypertable: %s, Query: %s", q.HumanLabel, q.HumanDescription, q.Hypertable, q.SqlQuery)
}
