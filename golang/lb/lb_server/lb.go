package main

import (
	"context"
	"io"
	"sort"

	"fmt"
	"log"
	"github.com/globulario/services/golang/lb/lbpb"
)


//*
// Return the list of servers in order of availability (lower loaded at first).
func (server *server) GetCanditates(ctx context.Context, rqst *lbpb.GetCanditatesRequest) (*lbpb.GetCanditatesResponse, error) {

	// The response channal.
	canditates := make(chan []*lbpb.ServerInfo)

	rqst_ := map[string]interface{}{"ServiceName": rqst.ServiceName, "Candidates": canditates}

	server.lb_get_candidates_info_channel <- rqst_

	// That will return the list of candidates. (or an empty list if no candidate or services was found.
	return &lbpb.GetCanditatesResponse{
		Servers: <-canditates,
	}, nil
}

//*
// Report load to the load balancer from the client.
func (server *server) ReportLoadInfo(stream lbpb.LoadBalancingService_ReportLoadInfoServer) error {

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream... The client have close the stream.
			stream.SendAndClose(&lbpb.ReportLoadInfoResponse{})

			// here I will remove the server form the list of candidates.
			server.lb_remove_candidate_info_channel <- msg.GetInfo().GetServerInfo()

			break
		} else if err != nil {
			return err
		} else {
			// Here I will process the request.
			server.lb_load_info_channel <- msg.GetInfo()
		}
	}

	return nil // nothing to do here.
}

// Sort
type By func(l1, l2 *lbpb.LoadInfo) bool

func (by By) Sort(loads []*lbpb.LoadInfo) {
	ps := &loadSorter{
		loads: loads,
		by:    by,
	}
	sort.Sort(ps)
}

type loadSorter struct {
	loads []*lbpb.LoadInfo
	by    func(l1, l2 *lbpb.LoadInfo) bool
}

func (s *loadSorter) Len() int {
	return len(s.loads)
}

func (s *loadSorter) Swap(i, j int) {
	s.loads[i], s.loads[j] = s.loads[j], s.loads[i]
}

func (s *loadSorter) Less(i, j int) bool {
	return s.by(s.loads[i], s.loads[j])
}

// The load balancing function.
func (server *server) startLoadBalancing() {
	fmt.Println("start load balancing")

	// Here will create the action channel.
	server.lb_load_info_channel = make(chan *lbpb.LoadInfo)
	server.lb_remove_candidate_info_channel = make(chan *lbpb.ServerInfo)
	server.lb_get_candidates_info_channel = make(chan map[string]interface{})
	server.lb_stop_channel = make(chan bool)

	// Here I will keep the list of server by service name.
	loads := make(map[string][]*lbpb.LoadInfo)

	// Start processing load balancing message.
	go func() {
		for {
			select {
			case <-server.lb_stop_channel:
				log.Println("---> stop load balancer")
				server.lb_stop_channel <- true
				return

			// Report load balancing informations.
			case load_info := <-server.lb_load_info_channel:
				if load_info != nil {

					// Create the array if it not exist.
					if loads[load_info.ServerInfo.Name] == nil {
						loads[load_info.ServerInfo.Name] = make([]*lbpb.LoadInfo, 0)
					}

					// Test if the server info exist.
					exist := false

					// Here I will append all existing load info except the new one.
					if loads[load_info.ServerInfo.Name] != nil {
						for i := 0; i < len(loads[load_info.ServerInfo.Name]); i++ {
							if loads[load_info.ServerInfo.Name][i].GetServerInfo().GetId() == load_info.ServerInfo.Id {
								exist = true
								loads[load_info.ServerInfo.Name][i] = load_info
								break
							}
						}
					}

					if !exist {
						loads[load_info.ServerInfo.Name] = append(loads[load_info.ServerInfo.Name], load_info)
					}
				}
			// Remove the server from the list of candidate.
			case server_info := <-server.lb_remove_candidate_info_channel:
				//log.Println("----> remove server from canditate list ", server_info)
				lst := make([]*lbpb.LoadInfo, 0)

				// Here I will append all existing load info except the new one.
				if loads[server_info.Name] != nil {
					for i := 0; i < len(loads[server_info.Name]); i++ {
						if loads[server_info.Name][i].GetServerInfo().GetId() != server_info.Id {
							lst = append(lst, loads[server_info.Name][i])
						}
					}
				}

				loads[server_info.Name] = lst
				// Return the list of candidates for a given services.
			case rqst := <-server.lb_get_candidates_info_channel:
				canditates := make([]*lbpb.ServerInfo, 0)

				// From the list list of load info I will retreive the server info.
				loads_ := loads[rqst["ServiceName"].(string)]

				// Sort load, smallest on top.
				By(func(l0, l1 *lbpb.LoadInfo) bool {
					return l0.Load1 < l1.Load1
				}).Sort(loads_)

				for i := 0; i < len(loads_); i++ {
					canditate := loads_[i].GetServerInfo()
					canditates = append(canditates, canditate)
				}

				// push the first node at last to distribute the load in case all
				// load are equal (that the case for a computer with multiple service instance).
				loads[rqst["ServiceName"].(string)] = append(loads_[1:], loads_[0])
				rqst["Candidates"].(chan []*lbpb.ServerInfo) <- canditates
			}
		}
	}()

}
