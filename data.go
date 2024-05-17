package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	srvConfig "github.com/CHESSComputing/golib/config"
	mongo "github.com/CHESSComputing/golib/mongo"
	utils "github.com/CHESSComputing/golib/utils"
)

// helper function to validate input data record against schema
func validateData(sname string, rec map[string]any) error {
	if smgr, ok := _smgr.Map[sname]; ok {
		schema := smgr.Schema
		err := schema.Validate(rec)
		if err != nil {
			return err
		}
	} else {
		msg := fmt.Sprintf("No schema '%s' found for your record", sname)
		log.Printf("ERROR: %s, schema map %+v", msg, _smgr.Map)
		return errors.New(msg)
	}
	return nil
}

// helper function to preprocess given record
/*
func preprocess(rec map[string]any) map[string]any {
	r := make(map[string]any)
	for k, v := range rec {
		switch val := v.(type) {
		case string:
			r[strings.ToLower(k)] = strings.ToLower(val)
		case []string:
			var vals []string
			for _, vvv := range val {
				vals = append(vals, strings.ToLower(vvv))
			}
			r[strings.ToLower(k)] = vals
		case []interface{}:
			var vals []string
			for _, vvv := range val {
				s := fmt.Sprintf("%v", vvv)
				vals = append(vals, strings.ToLower(s))
			}
			r[strings.ToLower(k)] = vals
		default:
			r[strings.ToLower(k)] = val
		}
	}
	return r
}
*/

// helper function to insert data into backend DB
func insertData(sname string, rec map[string]any, attrs, sep, div string) (string, error) {
	// load our schema
	if _, err := _smgr.Load(sname); err != nil {
		msg := fmt.Sprintf("unable to load %s error %v", sname, err)
		log.Println("ERROR: ", msg)
		return "", errors.New(msg)
	}
	if Verbose > 0 {
		log.Printf("insert data %+v", rec)
	}

	// check if data satisfies to one of the schema
	if err := validateData(sname, rec); err != nil {
		return "", err
	}
	if _, ok := rec["date"]; !ok {
		rec["date"] = time.Now().Unix()
	}
	rec["schema_file"] = sname
	rec["schema"] = schemaName(sname)
	// main attributes to work with
	var path string
	if v, ok := rec["data_location_raw"]; ok {
		path = v.(string)
	} else {
		path = filepath.Join("/tmp", os.Getenv("USER")) // for testing purposes
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Printf("Directory %s does not exist, will use /tmp", path)
			path = "/tmp"
		}
	}
	// generate unique id
	didValue, ok := rec["did"]
	did := fmt.Sprintf("%s", didValue)
	if !ok || did == "" {
		// create did out of provided attributes
		did = utils.CreateDID(rec, attrs, sep, div)
		rec["did"] = did
	}

	// check if given path exist on file system
	_, err := os.Stat(path)
	if err == nil {
		rec["path"] = path
		// add record to mongo DB
		var records []map[string]any
		records = append(records, rec)
		err = mongo.Upsert(
			srvConfig.Config.CHESSMetaData.MongoDB.DBName,
			srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
			"did", records)
		if err != nil {
			log.Printf("ERROR: unable to MongoUpsert for did=%s path=%s, error=%v", did, path, err)
		}
		return did, err
	}
	msg := fmt.Sprintf("No files found associated with DataLocationRaw=%s, error=%v", path, err)
	log.Printf("ERROR: %s", msg)
	return did, errors.New(msg)
}
