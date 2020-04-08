package gateway

import (
	"errors"
	"log"
	"sync"
)

type Multicaster interface {
	Multicast(request *MulticastRequest) (*MulticastResponse, error)
}

type MulticastRequest struct {
	Call          string
	HubSessionKey string
	ServerIDs     []int64
	ArgsByServer  map[int64][]interface{}
}

type multicaster struct {
	uyuniServerCallExecutor UyuniServerCallExecutor
	session                 Session
}

func NewMulticaster(uyuniServerCallExecutor UyuniServerCallExecutor, session Session) *multicaster {
	return &multicaster{uyuniServerCallExecutor, session}
}

func (m *multicaster) Multicast(request *MulticastRequest) (*MulticastResponse, error) {
	hubSession := m.session.RetrieveHubSession(request.HubSessionKey)
	if hubSession == nil {
		log.Printf("HubSession was not found. HubSessionKey: %v", request.HubSessionKey)
		return nil, errors.New("Authentication error: provided session key is invalid")
	}
	multicastCallRequest, err := m.generateMulticastCallRequest(request.Call, hubSession.ServerSessions, request.ServerIDs, request.ArgsByServer)
	if err != nil {
		return nil, err
	}
	return executeCallOnServers(multicastCallRequest), nil
}

type multicastCallRequest struct {
	call            serverCall
	serverCallInfos []serverCallInfo
}
type serverCallInfo struct {
	serverID int64
	endpoint string
	args     []interface{}
}
type serverCall func(endpoint string, args []interface{}) (interface{}, error)

func (m *multicaster) generateMulticastCallRequest(call string, serverSessions map[int64]*ServerSession, serverIDs []int64, argsByServer map[int64][]interface{}) (*multicastCallRequest, error) {
	callFunc := func(endpoint string, args []interface{}) (interface{}, error) {
		return m.uyuniServerCallExecutor.ExecuteCall(endpoint, call, args)
	}

	serverCallInfos := make([]serverCallInfo, 0, len(argsByServer))
	for _, serverID := range serverIDs {
		if serverSession, ok := serverSessions[serverID]; ok {
			args := append([]interface{}{serverSession.serverSessionKey}, argsByServer[serverID]...)
			serverCallInfos = append(serverCallInfos, serverCallInfo{serverID, serverSession.serverAPIEndpoint, args})
		} else {
			log.Printf("ServerSession was not found. ServerID: %v", serverID)
			return nil, errors.New("Authentication error: provided session key is invalid")
		}
	}
	return &multicastCallRequest{callFunc, serverCallInfos}, nil
}

type MulticastResponse struct {
	SuccessfulResponses map[int64]ServerSuccessfulResponse
	FailedResponses     map[int64]ServerFailedResponse
}
type ServerSuccessfulResponse struct {
	ServerID int64
	endpoint string
	Response interface{}
}
type ServerFailedResponse struct {
	ServerID     int64
	endpoint     string
	ErrorMessage string
}

func executeCallOnServers(multicastCallRequest *multicastCallRequest) *MulticastResponse {
	var mutexForSuccesfulResponses = &sync.Mutex{}
	var mutexForFailedResponses = &sync.Mutex{}

	successfulResponses := make(map[int64]ServerSuccessfulResponse)
	failedResponses := make(map[int64]ServerFailedResponse)

	var wg sync.WaitGroup
	wg.Add(len(multicastCallRequest.serverCallInfos))

	for _, serverCallInfo := range multicastCallRequest.serverCallInfos {
		go func(call serverCall, endpoint string, args []interface{}, serverID int64) {
			defer wg.Done()
			response, err := call(endpoint, args)
			if err != nil {
				mutexForFailedResponses.Lock()
				failedResponses[serverID] = ServerFailedResponse{serverID, endpoint, err.Error()}
				mutexForFailedResponses.Unlock()
			} else {
				mutexForSuccesfulResponses.Lock()
				successfulResponses[serverID] = ServerSuccessfulResponse{serverID, endpoint, response}
				mutexForSuccesfulResponses.Unlock()
			}
		}(multicastCallRequest.call, serverCallInfo.endpoint, serverCallInfo.args, serverCallInfo.serverID)
	}
	wg.Wait()
	return &MulticastResponse{successfulResponses, failedResponses}
}
