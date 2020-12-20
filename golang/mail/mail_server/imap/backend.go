package imap

import (
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
)

type Backend_impl struct {
}

func (self *Backend_impl) Login(connInfo *imap.ConnInfo, username, password string) (backend.User, error) {

	// I will use the datastore to authenticate the user.
	connection_id := username + "_db"
	log.Println("---> try to authenticate ", username, Backend_address)
	err := Store.CreateConnection(connection_id, connection_id, Backend_address, float64(Backend_port), 0, username, password, 5000, "", false)
	if err != nil {
		log.Println("fail to login: ", connection_id, username, password, err)
		return nil, err
	}

	// retreive account info.
	query := `{"name":"` + username + `"}`
	info, err := Store.FindOne("local_ressource", "local_ressource", "Accounts", query, "")
	if err != nil {
		log.Println("fail to authenticate with error: ", err)
		return nil, err
	}

	log.Println(username, " is now authenticated!")
	user := new(User_impl)
	user.info = info
	return user, nil
}
