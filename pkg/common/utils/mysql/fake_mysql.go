package mysql

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

var (
	fakeDB    *sqlx.DB
	mysqlMock sqlmock.Sqlmock
)

func newFakeDB() (*DB, error) {
	mysqlDb, m, _ := sqlmock.New()
	fakeDB = &sqlx.DB{
		DB: mysqlDb,
	}
	mysqlMock = m
	return &DB{DB: fakeDB}, nil
}
