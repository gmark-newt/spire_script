package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"net/http"
	"encoding/csv"
	"io"
	"io/ioutil"
	"encoding/json"
	"github.com/NewtopiaCI/common/log"
	"github.com/NewtopiaCI/common/models"
	"github.com/NewtopiaCI/common/database"
	jp "github.com/dustin/go-jsonpointer"
	"bytes"
)

type CommunityCSVTable struct {
	UserId      string    `json:"source_member_id"`  	// col 0 - source_member_id
	GroupToken  string    `json:"access_code"`  		// col 1 - access_code
	GroupName  	string    `json:"-"`  					// col 2
	FirstName	string    `json:"first_name"`  			// col 3
	LastName	string    `json:"last_name"`  			// col 4
	Email		string    `json:"email"`  				// col 5
	Gender		string    `json:"gender"`  				// col 6
	BirthDate	string    `json:"birth_date"`  			// col 7
	Profile		string    `json:"profile"`  			// col 8
	Password	string    `json:"password"`  			// col 9
}

func init() {
	//Set up logging connection for common/log
	// configLog := log.LogConfiguration{
	// 	Tag:       "producer_spire_script",
	// 	Network:   "tcp",
	// 	DBint:     1,
	// 	LogServer: "192.168.99.100:6379",
	// 	IsDebug:   false,
	// 	Version:   "1",
	// }
	// log.SetConfiguration(configLog)

	// Set up DB connection for common/database, as models.User functions use that configuration
	dbConfig := database.DBConfiguration{
		Host: 		"localhost",
		Port: 		5432,
		SSLMode: 	"disable",
		User:     	"devadm",
		Password:   "cHangeIT",
		Database:   "devlocal_app",
	}
	database.SetAppDatabase(dbConfig)
}

func main(){
	log.Print("Start Script")
	extractFile("spire_test_load.csv")
}

func extractFile(filename string){
	log.Print("Spire extraction script: Start Extracting File")
	lineCount := 0

	//Check if CSV file
	ext := filepath.Ext(filename)
	if ext != ".csv" {
		err := errors.New("Error: Input file is not .csv")
		log.Print("Spire extraction script error: ", err)
		return
	}

	file, err := os.Open(filename)
	if err != nil {
		log.Print("Spire extraction script error: ", err)
		return
	}
	defer file.Close()

	r := csv.NewReader(file)

	for i := 0; ; i++ {
		var data CommunityCSVTable
		lineCount = i

		row, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Print("Spire extraction script error: ", err)
			return
		}

		// Skip first row (headings)
		if i == 0 {
			continue
		}

		data = CommunityCSVTable{
			UserId:     row[0],
			GroupToken:	row[1],
			GroupName:  row[2],
			FirstName:  row[3],
			LastName:	row[4],
			Email:  	row[5],
			Gender:     row[6],
			BirthDate:  row[7],
			Profile:	row[8],
			Password:  	row[9],
		}

		//Grab Authorization Token from Spire Endpoint
		var record = make(map[string]interface{})
		record["first_name"] = data.FirstName
		record["last_name"] = data.LastName
		record["email"] = data.Email
		record["gender"] = data.Gender
		record["birth_date"] = data.BirthDate
		record["profile"] = data.Profile
		record["password"] = data.Password
		record["source_member_id"] = data.UserId
		record["access_code"] = data.GroupToken

		authToken, err := provisionSpireUser(record)
		if err != nil{
			log.Print("Spire extraction script error: ", err)
			continue
		}

		//Insert entry into spire_users
		var newUserId models.UUID
		newUserId.Parse(data.UserId)

		spireEntry := models.SpireUsers{
			UserId: newUserId,
			GroupToken: data.GroupToken,
			GroupName: data.GroupName,
			SpireAuthToken: authToken,
		}

		err = database.App.Save(&spireEntry).Error
		if err != nil {
			log.Print("Spire extraction script error: ", err)
			continue
		}
	}

	log.Print("Spire extraction script: Finished ", lineCount, " lines.")
	return
}

func provisionSpireUser(record map[string]interface{}) (string, error){

	var body = make(map[string]interface{})
	body["user"] = record
	body["client_id"] = "3b589mploshagqk6wwwa2owbu"
	body["client_secret"] = "7dusaw514t1g9n8b8jia41cto"

	log.Print(body)

	jsonData, jsonErr := json.Marshal(body)
	if jsonErr != nil {
		return "", jsonErr
	}

	log.Print(string(jsonData))

	client := &http.Client{}
	req, err := http.NewRequest("POST", "http://api.staging.spire.me/users", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/json")
	response, err := client.Do(req)

    if err != nil {
        return "", err
    } 
    defer response.Body.Close()

    contents, err := ioutil.ReadAll(response.Body)
    if err != nil {
        return "", err
    }
    log.Print("RAW: ", string(contents))

    metaMap := map[string]interface{}{}
	if err := json.Unmarshal(contents, &metaMap); err != nil {
		return "", err
	}

	value := jp.Get(metaMap, "/access_token")
	if value == nil{
		return "", fmt.Errorf("access_token is not found in JSON object from Spire Endpoint")
	}

	return value.(string), nil
}


