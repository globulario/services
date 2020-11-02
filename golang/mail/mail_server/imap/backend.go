package imap

import (
	"log"

	"encoding/json"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
)

type Backend_impl struct {
}

func (self *Backend_impl) Login(connInfo *imap.ConnInfo, username, password string) (backend.User, error) {

	// I will use the datastore to authenticate the user.
	connection_id := username + "_db"

	err := Store.CreateConnection(connection_id, connection_id, Backend_address, float64(Backend_port), 0, username, password, 5000, "", false)
	if err != nil {
		log.Println("fail to login: ", connection_id, username, password, err)
		return nil, err
	}

	// retreive account info.
	query := `{"name":"` + username + `"}`
	str, err := Store.FindOne("local_ressource", "local_ressource", "Accounts", query, "")
	if err != nil {
		return nil, err
	}

	info := make(map[string]interface{})
	err = json.Unmarshal([]byte(str), &info)
	if err != nil {
		return nil, err
	}

	user := new(User_impl)
	user.info = info
	return user, nil
}
