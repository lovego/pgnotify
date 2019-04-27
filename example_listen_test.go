package pglistener_test

import (
	"fmt"
	"time"

	"github.com/lovego/errs"
	"github.com/lovego/pglistener"
)

type testHandler struct {
}

func (h testHandler) ConnLoss(table string) {
	fmt.Printf("ConnLoss %s\n", table)
}

func (h testHandler) Create(table string, newBuf []byte) {
	fmt.Printf("Create %s\n  %s\n", table, newBuf)
}

func (h testHandler) Update(table string, oldBuf, newBuf []byte) {
	fmt.Printf("Update %s\n  old: %s\n  new: %s\n", table, oldBuf, newBuf)
}

func (h testHandler) Delete(table string, oldBuf []byte) {
	fmt.Printf("Delete %s\n  %s\n", table, oldBuf)
}

func ExampleListener_Listen() {
	testCreateUpdateDelete("students")
	testCreateUpdateDelete("public.students")

	// Output:
	// ConnLoss public.students
	// Create public.students
	//   {"id": 1, "name": "李雷", "time": "2018-09-08"}
	// Update public.students
	//   old: {"id": 1, "name": "李雷", "time": "2018-09-08"}
	//   new: {"id": 1, "name": "韩梅梅", "time": "2018-09-09"}
	// Delete public.students
	//   {"id": 1, "name": "韩梅梅", "time": "2018-09-10"}
	// ConnLoss public.students
	// Create public.students
	//   {"id": 1, "name": "李雷", "time": "2018-09-08"}
	// Update public.students
	//   old: {"id": 1, "name": "李雷", "time": "2018-09-08"}
	//   new: {"id": 1, "name": "韩梅梅", "time": "2018-09-09"}
	// Delete public.students
	//   {"id": 1, "name": "韩梅梅", "time": "2018-09-10"}
}

func testCreateUpdateDelete(table string) {
	createStudentsTable()

	listener, err := pglistener.New(dbUrl, logger)
	if err != nil {
		fmt.Println(errs.WithStack(err))
		return
	}
	if err := listener.Listen(
		table,
		"$1.id, $1.name, to_char($1.time, 'YYYY-MM-DD') as time",
		"$1.id, $1.name",
		testHandler{},
	); err != nil {
		panic(errs.WithStack(err))
	}

	// from now on, testHandler will be notified on INSERT / UPDATE / DELETE.
	if _, err := testDB.Exec(`
    INSERT INTO students(name, time) VALUES ('李雷', '2018-09-08 15:55:00+08')
  `); err != nil {
		panic(err)
	}
	if _, err = testDB.Exec(`
    UPDATE students SET name = '韩梅梅', time = '2018-09-09 15:56:00+08'
  `); err != nil {
		panic(err)
	}
	// this one should not be notified
	if _, err = testDB.Exec(`
    UPDATE students SET time = '2018-09-10 15:57:00+08'
  `); err != nil {
		panic(err)
	}
	if _, err = testDB.Exec(`DELETE FROM students`); err != nil {
		panic(err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := listener.Unlisten(table); err != nil {
		panic(err)
	}
}

func createStudentsTable() {
	if _, err := testDB.Exec(`
	DROP TABLE IF EXISTS students;
	CREATE TABLE IF NOT EXISTS students (
		id   bigserial,
		name varchar(100),
		time timestamptz,
    other text default ''
	)`); err != nil {
		panic(err)
	}
}