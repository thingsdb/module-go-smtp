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
//     smptp.send_mail(["alice@foo.bar"], {
//         from: "bob@foo.bar",
//         subject: "my subject",
//         plain: "my body",
//     }).else(|err| {
//         // error handling....
//     }));
//
package main

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"sync"

	mailyak "github.com/cesbit/mailyak/v3"
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

type mailObj struct {
	Bcc      []string `msgpack:"bcc"`
	Cc       []string `msgpack:"cc"`
	From     *string  `msgpack:"from"`
	FromName *string  `msgpack:"from_name"`
	HTML     *string  `msgpack:"html"`
	Plain    *string  `msgpack:"plain"`
	Replyto  *string  `msgpack:"reply_to"`
	Subject  *string  `msgpack:"subject"`
}

type mailReq struct {
	To   []string `msgpack:"to"`
	Mail *mailObj `msgpack:"mailobj"`
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
	var req mailReq
	err := msgpack.Unmarshal(pkg.Data, &req)
	if err != nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExBadData,
			"Failed to unpack SMTP request")
		return
	}

	email := mailyak.New(conn.Host, conn.Auth)
	if req.To != nil && len(req.To) > 0 {
		email.To(req.To...)
	} else {
		timod.WriteEx(
			pkg.Pid,
			timod.ExBadData,
			"At least one `to` address is required")
		return
	}

	if req.Mail == nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExBadData,
			"A mail object is required")
		return
	}

	if req.Mail.Subject != nil {
		email.Subject(*req.Mail.Subject)
	} else {
		timod.WriteEx(
			pkg.Pid,
			timod.ExBadData,
			"Mail subject is missing")
		return
	}

	if req.Mail.Bcc != nil {
		email.Bcc(req.Mail.Bcc...)
	}

	if req.Mail.Cc != nil {
		email.Cc(req.Mail.Cc...)
	}

	if req.Mail.Replyto != nil {
		email.ReplyTo(*req.Mail.Replyto)
	}

	if req.Mail.From != nil {
		email.From(*req.Mail.From)
	}

	if req.Mail.FromName != nil {
		email.FromName(*req.Mail.FromName)
	}

	if req.Mail.Plain != nil {
		email.Plain().Set(*req.Mail.Plain)
	}

	if req.Mail.HTML != nil {
		email.HTML().Set(*req.Mail.HTML)
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
