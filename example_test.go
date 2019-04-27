package pglistener_test

import (
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/lovego/bsql"
	loggerPkg "github.com/lovego/logger"
	"github.com/lovego/maps"
	"github.com/lovego/pglistener"
	"github.com/lovego/pglistener/pghandler"
)

var dbUrl = getTestDataSource()
var testDB = connectDB(dbUrl)
var logger = loggerPkg.New(os.Stderr)

type Student struct {
	Id    int64
	Name  string
	Class string
}

func ExampleListener() {
	initStudentsTable()

	var studentsMap = make(map[int64]Student)
	var classesMap = make(map[string][]Student)

	listener, err := pglistener.New(dbUrl, logger)
	if err != nil {
		panic(err)
	}
	if err := listener.ListenTable(getTableHandler(&studentsMap, &classesMap)); err != nil {
		panic(err)
	}

	// from now on, studentsMap and classesMap is always synchronized with students table.
	fmt.Println(`init:`)
	maps.Println(studentsMap)
	maps.Println(classesMap)

	// even you insert some rows.
	if _, err := testDB.Exec(`
INSERT INTO students (id, name, class)
VALUES
(3, 'Lily',   '初三2班'),
(4, 'Lucy',   '初三2班');
`); err != nil {
		panic(err)
	}
	time.Sleep(10 * time.Millisecond)
	fmt.Println(`after INSERT:`)
	maps.Println(studentsMap)
	maps.Println(classesMap)

	// even you update some rows.
	if _, err := testDB.Exec(`UPDATE students SET class = '初三2班'`); err != nil {
		panic(err)
	}
	time.Sleep(10 * time.Millisecond)
	fmt.Println(`after UPDATE:`)
	maps.Println(studentsMap)
	maps.Println(classesMap)

	// even you delete some rows.
	if _, err := testDB.Exec(`DELETE FROM students WHERE id in (3, 4)`); err != nil {
		panic(err)
	}
	time.Sleep(10 * time.Millisecond)
	fmt.Println(`after DELETE:`)
	maps.Println(studentsMap)
	maps.Println(classesMap)

	// Output:
	// init:
	// map[1:{1 李雷 初三1班} 2:{2 韩梅梅 初三1班}]
	// map[初三1班:[{1 李雷 初三1班} {2 韩梅梅 初三1班}]]
	// after INSERT:
	// map[1:{1 李雷 初三1班} 2:{2 韩梅梅 初三1班} 3:{3 Lily 初三2班} 4:{4 Lucy 初三2班}]
	// map[初三1班:[{1 李雷 初三1班} {2 韩梅梅 初三1班}] 初三2班:[{3 Lily 初三2班} {4 Lucy 初三2班}]]
	// after UPDATE:
	// map[1:{1 李雷 初三2班} 2:{2 韩梅梅 初三2班} 3:{3 Lily 初三2班} 4:{4 Lucy 初三2班}]
	// map[初三2班:[{1 李雷 初三2班} {2 韩梅梅 初三2班} {3 Lily 初三2班} {4 Lucy 初三2班}]]
	// after DELETE:
	// map[1:{1 李雷 初三2班} 2:{2 韩梅梅 初三2班}]
	// map[初三2班:[{1 李雷 初三2班} {2 韩梅梅 初三2班}]]
}

func initStudentsTable() {
	if _, err := testDB.Exec(`
DROP TABLE IF EXISTS students;
CREATE TABLE IF NOT EXISTS students (
  id    bigserial,
  name  text,
  class text
);
INSERT INTO students (id, name, class)
VALUES
(1, '李雷',   '初三1班'),
(2, '韩梅梅', '初三1班');
`); err != nil {
		panic(err)
	}
}

func getTableHandler(studentsMap, classesMap interface{}) pglistener.TableHandler {
	var mutex sync.RWMutex

	return pghandler.New(pghandler.Table{Name: "students"}, Student{}, []pghandler.Data{
		{
			RWMutex: &mutex, MapPtr: studentsMap, MapKeys: []string{"Id"},
		}, {
			RWMutex: &mutex, MapPtr: classesMap, MapKeys: []string{"Class"},
			SortedSetUniqueKey: []string{"Id"},
		},
	}, bsql.New(testDB, time.Second), logger)
}

func getTestDataSource() string {
	if env := os.Getenv("PG_DATA_SOURCE"); env != "" {
		return env
	} else if runtime.GOOS == "darwin" {
		return "postgres://postgres:@localhost/test?sslmode=disable"
	} else {
		return "postgres://travis:123456@localhost:5433/travis?sslmode=disable"
	}
}

func connectDB(dbUrl string) *sql.DB {
	db, err := sql.Open(`postgres`, dbUrl)
	if err != nil {
		panic(err)
	}
	return db
}