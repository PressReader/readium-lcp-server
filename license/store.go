// Copyright 2017 European Digital Reading Lab. All rights reserved.
// Licensed to the Readium Foundation under one or more contributor license agreements.
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file exposed on Github (readium) in the project repository.

package license

import (
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/readium/readium-lcp-server/config"
)

var NotFound = errors.New("License not found")

type Store interface {
	//List() func() (License, error)
	List(ContentId string, page int, pageNum int) func() (LicenseReport, error)
	ListAll(page int, pageNum int) func() (LicenseReport, error)
	UpdateRights(l License) error
	Update(l License) error
	UpdateLsdStatus(id string, status int32) error
	Add(l License) error
	Get(id string) (License, error)
}

type sqlStore struct {
	db              *sql.DB
	listall         *sql.Stmt
	list            *sql.Stmt
	updaterights    *sql.Stmt
	add             *sql.Stmt
	update          *sql.Stmt
	updatelsdstatus *sql.Stmt
	get             *sql.Stmt
}

// ListAll lists all licenses in ante-chronological order
// pageNum starts at 0
//
func (s *sqlStore) ListAll(page int, pageNum int) func() (LicenseReport, error) {
	listLicenses, err := s.listall.Query(page, pageNum*page)
	if err != nil {
		return func() (LicenseReport, error) { return LicenseReport{}, err }
	}
	return func() (LicenseReport, error) {
		var l LicenseReport
		l.User = UserInfo{}
		l.Rights = new(UserRights)
		if listLicenses.Next() {
			err := listLicenses.Scan(&l.Id, &l.User.Id, &l.Provider, &l.Issued, &l.Updated,
				&l.Rights.Print, &l.Rights.Copy, &l.Rights.Start, &l.Rights.End, &l.ContentId)

			if err != nil {
				return l, err
			}

		} else {
			listLicenses.Close()
			err = NotFound
		}
		return l, err
	}
}

// List lists licenses for a given ContentId
// pageNum starting at 0
//
func (s *sqlStore) List(contentID string, page int, pageNum int) func() (LicenseReport, error) {
	listLicenses, err := s.list.Query(contentID, page, pageNum*page)
	if err != nil {
		return func() (LicenseReport, error) { return LicenseReport{}, err }
	}
	return func() (LicenseReport, error) {
		var l LicenseReport
		l.User = UserInfo{}
		l.Rights = new(UserRights)
		if listLicenses.Next() {

			err := listLicenses.Scan(&l.Id, &l.User.Id, &l.Provider, &l.Issued, &l.Updated,
				&l.Rights.Print, &l.Rights.Copy, &l.Rights.Start, &l.Rights.End, &l.ContentId)
			if err != nil {
				return l, err
			}
		} else {
			listLicenses.Close()
			err = NotFound
		}
		return l, err
	}
}

// UpdateRights
//
func (s *sqlStore) UpdateRights(l License) error {
	result, err := s.updaterights.Exec(l.Rights.Print, l.Rights.Copy, l.Rights.Start, l.Rights.End, time.Now().UTC().Truncate(time.Second), l.Id)

	if err == nil {
		if r, _ := result.RowsAffected(); r == 0 {
			return NotFound
		}
	}
	return err
}

// Add creates a new record in the license table
//
func (s *sqlStore) Add(l License) error {
	_, err := s.add.Exec(
		l.Id, l.User.Id, l.Provider, l.Issued, nil,
		l.Rights.Print, l.Rights.Copy, l.Rights.Start, l.Rights.End,
		l.ContentId)
	return err
}

// Update updates a record in the license table
//
func (s *sqlStore) Update(l License) error {
	_, err := s.update.Exec(
		l.User.Id, l.Provider,
		time.Now().UTC().Truncate(time.Second),
		l.Rights.Print, l.Rights.Copy, l.Rights.Start, l.Rights.End,
		l.ContentId,
		l.Id)

	return err
}

// UpdateLsdStatus
//
func (s *sqlStore) UpdateLsdStatus(id string, status int32) error {
	_, err := s.updatelsdstatus.Exec(
		status,
		id)

	return err
}

// Get a license from the db
//
func (s *sqlStore) Get(id string) (License, error) {
	// create an empty license, add user rights
	var l License
	l.Rights = new(UserRights)

	row := s.get.QueryRow(id)

	err := row.Scan(&l.Id, &l.User.Id, &l.Provider, &l.Issued, &l.Updated,
		&l.Rights.Print, &l.Rights.Copy, &l.Rights.Start, &l.Rights.End,
		&l.ContentId)

	if err != nil {
		if err == sql.ErrNoRows {
			return l, NotFound
		} else {
			return l, err
		}
	}

	return l, nil
}

// NewSqlStore
//
func NewSqlStore(db *sql.DB) (Store, error) {
	
	var tabledefquery, listallquery, listquery, updaterightsquery, addquery, updatequery, updatelsdstatusquery, getquery string

	if strings.HasPrefix(config.Config.LcpServer.Database, "postgres") {
		// postgres
		tabledefquery = tableDefPostgers
		listallquery = `SELECT id, user_id, provider, issued, updated,
			rights_print, rights_copy, rights_start, rights_end, content_fk
			FROM license
			ORDER BY issued desc LIMIT $1 OFFSET $2`
		listquery = `SELECT id, user_id, provider, issued, updated,
			rights_print, rights_copy, rights_start, rights_end, content_fk
			FROM license
			WHERE content_fk=$1 LIMIT $2 OFFSET $3`
		updaterightsquery = "UPDATE license SET rights_print=$1, rights_copy=$2, rights_start=$3, rights_end=$4, updated=$5 WHERE id=$6"
		addquery = `INSERT INTO license (id, user_id, provider, issued, updated,
			rights_print, rights_copy, rights_start, rights_end, content_fk) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
		updatequery = `UPDATE license SET user_id=$1, provider=$2, updated=$3,
			rights_print=$4, rights_copy=$5, rights_start=$6, rights_end=$7, content_fk =$8
			WHERE id=$9`
		updatelsdstatusquery = `UPDATE license SET lsd_status =$1 WHERE id=$2`
		getquery = `SELECT id, user_id, provider, issued, updated, rights_print, rights_copy,
			rights_start, rights_end, content_fk FROM license
			where id = $1`
	}else{
		// mysql/sqlite
		tabledefquery = tableDef
		listallquery = `SELECT id, user_id, provider, issued, updated,
			rights_print, rights_copy, rights_start, rights_end, content_fk
			FROM license
			ORDER BY issued desc LIMIT ? OFFSET ?`
		listquery = `SELECT id, user_id, provider, issued, updated,
			rights_print, rights_copy, rights_start, rights_end, content_fk
			FROM license
			WHERE content_fk=? LIMIT ? OFFSET ?`
		updaterightsquery = "UPDATE license SET rights_print=?, rights_copy=?, rights_start=?, rights_end=?,u pdated=? WHERE id=?"
		addquery = `INSERT INTO license (id, user_id, provider, issued, updated,
			rights_print, rights_copy, rights_start, rights_end, content_fk) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		updatequery = `UPDATE license SET user_id=?, provider=?, updated=?,
			rights_print=?, rights_copy=?, rights_start=?, rights_end=?, content_fk =?
			WHERE id=?`
		updatelsdstatusquery = `UPDATE license SET lsd_status =? WHERE id=?`
		getquery = `SELECT id, user_id, provider, issued, updated, rights_print, rights_copy,
			rights_start, rights_end, content_fk FROM license
			where id = ?`
	}

	// if sqlite/postgres, create the license table if it does not exist
	if strings.HasPrefix(config.Config.LcpServer.Database, "sqlite") || strings.HasPrefix(config.Config.LcpServer.Database, "postgres") {
		_, err := db.Exec(tabledefquery)
		if err != nil {
			log.Println("Error creating license table")
			return nil, err
		}
	}

	listall, err := db.Prepare(listallquery)
	if err != nil {
		return nil, err
	}

	list, err := db.Prepare(listquery)
	if err != nil {
		return nil, err
	}

	updaterights, err := db.Prepare(updaterightsquery)
	if err != nil {
		return nil, err
	}

	add, err := db.Prepare(addquery)
	if err != nil {
		return nil, err
	}

	update, err := db.Prepare(updatequery)
	if err != nil {
		return nil, err
	}

	updatelsdstatus, err := db.Prepare(updatelsdstatusquery)
	if err != nil {
		return nil, err
	}

	get, err := db.Prepare(getquery)
	if err != nil {
		return nil, err
	}

	return &sqlStore{db, listall, list, updaterights, add, update, updatelsdstatus, get}, nil
}

const tableDef = "CREATE TABLE IF NOT EXISTS license (" +
	"id varchar(255) PRIMARY KEY," +
	"user_id varchar(255) NOT NULL," +
	"provider varchar(255) NOT NULL," +
	"issued datetime NOT NULL," +
	"updated datetime DEFAULT NULL," +
	"rights_print int(11) DEFAULT NULL," +
	"rights_copy int(11) DEFAULT NULL," +
	"rights_start datetime DEFAULT NULL," +
	"rights_end datetime DEFAULT NULL," +
	"content_fk varchar(255) NOT NULL," +
	"lsd_status integer default 0," +
	"FOREIGN KEY(content_fk) REFERENCES content(id))"

const tableDefPostgers = "CREATE TABLE IF NOT EXISTS license (" +
	"id VARCHAR(255) PRIMARY KEY," +
	"user_id VARCHAR(255) NOT NULL," +
	"provider VARCHAR(255) NOT NULL," +
	"issued TIMESTAMPTZ NOT NULL," +
	"updated TIMESTAMPTZ DEFAULT NULL," +
	"rights_print INT DEFAULT NULL," +
	"rights_copy INT DEFAULT NULL," +
	"rights_start TIMESTAMPTZ DEFAULT NULL," +
	"rights_end TIMESTAMPTZ DEFAULT NULL," +
	"content_fk VARCHAR(255) NOT NULL," +
	"lsd_status INT default 0," +
	"FOREIGN KEY(content_fk) REFERENCES content(id))"