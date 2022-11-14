// ThingsDB module for sending emails using SMTP.
//
// For example:
//
//     // Create the module (@thingsdb scope)
//     new_module('smtp', 'github.com/thingsdb/module-go-smtp');
//
//     // Configure the module
//     set_module_conf('smtp', {
//         host: 'my-smtp-host:587",
//         auth: ['my-optional-user', 'my-optional-password'],
//     });
//
//     // Use the module
//     smptp.send_mail("mychannel", {
//         title: "my title"
//     }).then(|sid| {
//         sid;  // the sid of the new ticket
//     });
//
package main

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"sync"

	mailyak "github.com/domodwyer/mailyak/v3"
	timod "github.com/thingsdb/go-timod"

	"github.com/vmihailenco/msgpack"
)

var mux sync.Mutex
var conn connSMTP

type confSMTP struct {
	Host string   `msgpack:"host"`
	Auth []string `msgpack:"auth"`
}

type connSMTP struct {
	Host string
	Auth smtp.Auth
}

type mailRequest struct {
	Bcc      []string `msgpack:bcc`
	Cc       []string `msgpack:cc`
	From     *string  `msgpack:from`
	FromName *string  `msgpack:from_name`
	HTML     *string  `msgpack:html`
	Plain    *string  `msgpack:plain`
	Replyto  *string  `msgpack:reply_to`
	Subject  *string  `msgpack:subject`
	To       []string `msgpack:"to"`
}

func handleConf(conf *confSMTP) error {
	if conf.Host == "" {
		return fmt.Errorf("SMTP Host must not be empty")
	}
	conn.Host = conf.Host
	if conf.Auth != nil {
		if len(conf.Auth) != 2 {
			return fmt.Errorf("SMTP Auth must be an array with a username and password")
		}

	}
	host := strings.Split(conf.Host, ":")[0]
	conn.Auth = smtp.PlainAuth("", conf.Auth[0], conf.Auth[1], host)
	return nil
}

func onModuleReq(pkg *timod.Pkg) {
	var req mailRequest
	err := msgpack.Unmarshal(pkg.Data, &req)
	if err != nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExBadData,
			"Failed to unpack SMTP request")
		return
	}

	email := mailyak.New(conn.Host, conn.Auth)
	if req.Bcc != nil {
		email.Bcc(req.Bcc...)
	}

	if err := email.Send(); err != nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExOperation,
			fmt.Sprintf("Failed to send mail: %s", err.Error()))
		return
	}

	timod.WriteResponse(pkg.Pid, nil)
}

func handler(buf *timod.Buffer, quit chan bool) {
	for {
		select {
		case pkg := <-buf.PkgCh:
			switch timod.Proto(pkg.Tp) {
			case timod.ProtoModuleConf:
				var conf confSMTP

				err := msgpack.Unmarshal(pkg.Data, &conf)
				if err != nil {
					log.Println("Missing or invalid SMTP configuration")
					timod.WriteConfErr()
					break
				}

				err = handleConf(&conf)
				if err != nil {
					log.Println(err.Error())
					timod.WriteConfErr()
					break
				}

				timod.WriteConfOk()

			case timod.ProtoModuleReq:
				onModuleReq(pkg)

			default:
				log.Printf("Unexpected package type: %d", pkg.Tp)
			}
		case err := <-buf.ErrCh:
			// In case of an error you probably want to quit the module.
			// ThingsDB will try to restart the module a few times if this
			// happens.
			log.Printf("Error: %s", err)
			quit <- true
		}
	}
}

func main() {
	// Starts the module
	timod.StartModule("smtp", handler)
}
