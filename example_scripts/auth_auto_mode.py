#!/usr/bin/python
import xmlrpclib  
import itertools

HUB_URL = "http://localhost:8888/hub/rpc/api"
HUB_LOGIN = "admin"
HUB_PASSWORD = "admin"

client = xmlrpclib.Server(HUB_URL, verbose=0)

hubSessionKey = client.hub.loginWithAutoconnectMode(HUB_LOGIN, HUB_PASSWORD)

#get the server IDs
serverIDs = client.hub.listServerIds(hubSessionKey)

#perform the required operation 
systemsPerServer = client.multicast.system.listSystems(hubSessionKey, serverIDs)
successfulResponses = systemsPerServer["Successful"]["Responses"]
failedResponses = systemsPerServer["Failed"]["Responses"]

for system in itertools.chain.from_iterable(successfulResponses):
    print(system)

#logout
client.auth.logout(hubSessionKey)