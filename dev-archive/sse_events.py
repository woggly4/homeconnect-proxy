import imp
import json
import sseclient
import requests
import pprint


def get_recent_change():
    url = 'http://localhost:8088/homeappliances/events'
    response = requests.get(url, stream=True)
    client = sseclient.SSEClient(response)
    for event in client.events():
        print(event.event)
        print(event.id)
        pprint.pprint(event.data)
    return

get_recent_change()