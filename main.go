package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path"

	_ "github.com/mattn/go-sqlite3"
	xkcd "github.com/nishanths/go-xkcd"
	"golang.org/x/xerrors"
)

func main() {
	u, err := user.Current()
	if err != nil {
		log.Fatal("Unable to get current user: ", err)
	}

	cfgDBFileName := u.HomeDir + `/.xkcd_downloader.db`

	imgDir := u.HomeDir + `/Pictures/xkcd`

	// create the directory if necessary
	err = os.MkdirAll(imgDir, os.ModePerm)
	if err != nil {
		log.Fatalf(`Unable to create directory %s: %v`, imgDir, err)
	}

	conn, err := NewDB(cfgDBFileName)
	if err != nil {
		log.Fatalf("NewDB failed: %v", err)
	}
	log.Printf(`DB: %s Local Directory: %s`, cfgDBFileName, imgDir)

	client := xkcd.NewClient()

	latest, err := client.Latest()
	if err != nil {
		log.Fatalf("xkcd client.Latest failed: %v", err)
	}

	fmt.Printf("%+v \n", latest)
	for id := 1; id <= latest.Number; id++ {
		if id == 404 {
			// there is no comic #404
			continue
		}
		var imageURL string
		err = conn.QueryRow(`select imageURL from comics where id=?`, id).Scan(&imageURL)
		if err != nil {
			if err != sql.ErrNoRows {
				// something terrible happened
				log.Fatalf(`QueryRow err %v`, err)
			}
			// need to check api to get filename and insert into db
			c, err := client.Get(id)
			if err != nil {
				log.Fatalf(`client.Get err %v`, err)
			}
			imageURL = c.ImageURL
			_, err = conn.Exec(`insert into comics(id, imageURL) values (?,?)`, id, imageURL)
		}
		// check if file exists, download if necessary
		localFilename := imgDir + `/` + path.Base(imageURL)
		if fileExists(localFilename) {
			continue
		}
		log.Printf("Downloading %d. %s to %s\n", id, imageURL, localFilename)
		err = download(imageURL, localFilename)
		if err != nil {
			log.Printf(`Unable to download %d. %s to %s`, id, imageURL, localFilename)
		}

	}
}

func download(url, local string) error {
	resp, err := http.Get("http://cnn.com")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fileBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(local, fileBytes, os.ModePerm)

	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

type db struct {
	*sql.DB
}

func NewDB(filename string) (*db, error) {
	conn, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, xerrors.Errorf(`Failed to open db "%s":  %v`, filename, err)
	}

	// init the tables if they aren't there already
	sqlStmt := `
		create table if not exists comics (id integer not null primary key, imageURL text);
		create table if not exists prefs (key text not null primary key, val text not null);
	`

	_, err = conn.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return nil, xerrors.Errorf(`db.Exec failed: %v`, err)
	}

	return &db{DB: conn}, nil
}
