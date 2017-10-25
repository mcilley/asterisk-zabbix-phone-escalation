package main

import (
	"github.com/matcornic/hermes"
	"fmt"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"container/ring"
	"time"
	"strconv"
	"sort"
	"gopkg.in/gomail.v2"
   	"crypto/tls"
   	"os"
	"log"
	"flag"
	"io/ioutil"
)

//Determin escalation/step based on numeric week of year
func getEscalation( step int, maxSteps int, escalations ring.Ring ) string{
	_ , week := time.Now().ISOWeek()
	return fmt.Sprintf( "%d",escalations.Move( step+week ).Value )
}

//get our contacts info (ids, etc)
func getContacts( db sql.DB ) (int,[]string){

	var steps int
	var conIds []string

	rows, err := db.Query( "SELECT COUNT(*) FROM contacts" )
	rows.Next()
	rows.Scan( &steps )
	rows.Close()

	//if there's an error print it out, yerp
	if err != nil {
		log.Fatalf( "There has been an error retrieving the number of contacts from our db" )
	}

	var id string
	rows, err = db.Query( "SELECT contact_id FROM contacts" )
	for rows.Next() {
		rows.Scan( &id )
		conIds = append( conIds, id )
	}
	rows.Close()

	//if there's an error print it out, yerp
	if err != nil {
		log.Fatalf( "There has been an error retrieving contacts" )
	}
	return steps, conIds
}

//get our contact number/name from our sqlite db
func getContactNum( db sql.DB, id string ) (string, string){

	var phone string
	var name string

	rows, _ := db.Query( "SELECT phone, first_name FROM contacts WHERE contact_id ="+id )
	rows.Next()
	rows.Scan( &phone, &name)
	return phone, name

	//if there's an error print it out, yerp
	if err != nil {
		log.Fatalf( "There has been an error retrieving contacts" )
	}	
}

//generate our escalation table for our hermes template thingy.. yeah, a bit janky
func genEscTable( db sql.DB, escOrder ring.Ring, conIds []string, steps int) [][]hermes.Entry{

	var escTable [][]hermes.Entry
	sort.Strings(conIds)
	for _, num := range conIds {

		var escy []hermes.Entry
		var esc hermes.Entry

        num_num, err := strconv.Atoi(num)
		
		if err != nil {
			log.Fatalf( "String to integer conversion error" )
		}	
        
        num_num = num_num - 1

       	esc_num := getEscalation(num_num, steps, escOrder ) 
       	pNum, name := getContactNum( db, esc_num )
		
		esc.Key = "Escalation Order"
		esc.Value = num
		escy = append( escy, esc)

		esc.Key = "Name"
		esc.Value = name
		escy = append( escy, esc)
		
		esc.Key = "Phone"
		esc.Value = pNum
		escy = append( escy, esc)

		escTable = append(escTable,escy)
	}
	return escTable
}

//generate our email (returns html string of email)
func genEmail( escTable [][]hermes.Entry ) string{

	h := hermes.Hermes{
		Product: hermes.Product{
	 		Name: "CTT",
	 		Link: "<link address for email>",
	 		Logo: "<logo address>",
	 	},
	}	
	email, _ :=h.GenerateHTML( genHtmlEmail(escTable) )
	return email
}

//generates email in HTML format
func genHtmlEmail( table [][]hermes.Entry ) hermes.Email {

	return hermes.Email{
		Body: hermes.Body{
			Name: "<Name of group>",
			Intros: []string{
				"The Current On-Call Rotation for this week is:",
			},
			Table: hermes.Table{Data:table},
		},
	}
}

func main() {

	if len(os.Args) < 1{
		log.Fatalf( "No args provided, nothing to do here!" )
	}

	message := flag.String( "message","","Required Field!!!! - Message/notifaction to be played to destination ex. \"Hey, Stuff is broken turkey\"" )
	alertId := flag.String( "alertId","0","zabbix alert ID, numeric ID for alert, used for acking alerts via api" )
	escalation := flag.Int( "escalation",0,"numeric tier of escalation (ex. 1)" )
	dest := flag.String( "dest","","Destination of call/notification (11 digit format ex.12223334444)" )
	name := flag.String( "name","","Name of Person that will be contacted" )
	emailsched := flag.Bool( "emailsched",false,"whether we send out an email with the oncall sched (true or false)" )
	outgoingDir := flag.String( "outgoingDir","/var/spool/asterisk/outgoing/","<astspooldir>/outgoing - The outgoing call file dir, where call files are put for processing" )
	contactsDb :=  flag.String( "contactsDb","/home/zabbix/asterisk/contacts.db","path+file which contains contacts for escalation" )

	flag.Parse()

	//check that our db exists
	if _, err := os.Stat(*contactsDb); os.IsNotExist(err) {
		log.Fatalf( "db does not exist!" )
	}

	//connect to our db
	db, err := sql.Open( "sqlite3", *contactsDb )

	//if there's an error print it out, yerp
	if err != nil {
		log.Fatalf( "There has been an error reading our sqlite db" )
	}

	//get our contact IDs and such
	steps, conIds := getContacts( *db )

	//this is our ring of contacts, woohoo
	escOrder := ring.New( len(conIds) )
	for i := 1; i <= escOrder.Len(); i++ {  
	    escOrder = escOrder.Next()
	    escOrder.Value = i
	}

	if *emailsched == true{

		escTable := genEscTable( *db, *escOrder, conIds, steps)
		emailBody := genEmail(escTable)
		m := gomail.NewMessage()
		m.SetHeader("From", "<from email addr>")
		m.SetHeader("To", "<to email addr>")
		m.SetHeader("Subject", "Team oncall schedule for this week")
		m.SetBody("text/html",emailBody)
		d := gomail.NewDialer("<smtp email server>",25,"","")
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		
		//send the email out
		if err := d.DialAndSend(m); err != nil {
		    fmt.Print( "Email_Error" )
		}else {
			fmt.Print( "Email_Sent" )
		}
		
	}else{

		//Deal with us not having a message to send or invalid escalation
		if *message == "" {
			log.Fatalf( "I have no error message to send!" )
		}else if *escalation >= len(conIds){
			log.Fatalf( "We're out of escalations!" )
		}

		esc_id := getEscalation( *escalation, steps, *escOrder )
		*dest, *name = getContactNum( *db, esc_id  )

		//fmt.Println("Channel: Motif/google/"+*dest+"@voice.google.com\nContext: outgoing-notify\nExtension: s\nPriority: 1\nSetvar: MESSAGE="+*message+"\nSetvar: ALERTID="+*alertId+"\nSetvar: ESCALATION="+strconv.FormatInt(int64(*escalation),10)+"\nSetvar: NAME="+*name )	
		
		//compose our call file, note documentation for format, variables are @ https://wiki.asterisk.org/wiki/display/AST/Asterisk+Call+Files
		call := []byte( "Channel: Motif/google/"+*dest+"@voice.google.com\nContext: outgoing-notify\nExtension: s\nPriority: 1\nSetvar: MESSAGE="+*message+"\nSetvar: ALERTID="+*alertId+"\nSetvar: ESCALATION="+strconv.FormatInt(int64(*escalation),10)+"\nSetvar: NAME="+*name )
		
		//write out our call file for placement into asterisk's call queue, file name is prepended with timestamp of creation
		err = ioutil.WriteFile( *outgoingDir+strconv.FormatInt( time.Now().Unix(),10 )+".call",call ,0644 )

		//if there's an error print it out, yerp
		if err != nil {
			log.Fatalf( "There has been an error writing out the call file for asterisk!" )
		}
	}
}
