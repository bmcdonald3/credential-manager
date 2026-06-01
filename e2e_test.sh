#!/bin/bash

export MASTER_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

echo "Seeding the encrypted credential store..."
go run seed.go

echo "Building the BMC Manager service..."
go build -o bmc-server ./cmd/server

echo "Starting the BMC Manager service in the background..."
./bmc-server serve --database-url="file:data.db?cache=shared&_fk=1" > server.log 2>&1 &
SERVER_PID=$!

echo "Waiting 5 seconds for the server to initialize..."
sleep 5

echo "Submitting the credential rotation request..."
curl -sS -X POST -H 'Content-Type: application/json' -d '{"metadata":{"name":"rotate-bmc-172"},"spec":{"targetAddress":"172.24.0.3","targetAccount":"3","secretId":"bmc-secret-172","rotationTrigger":"test-run-1"}}' http://localhost:8080/bmccredentials/

echo ""
echo "Waiting 10 seconds for the reconciliation loop to execute..."
sleep 10

echo "Retrieving the final BmcCredential status..."
curl -sS http://localhost:8080/bmccredentials/

echo ""
echo "Verifying Redfish endpoint with NEW credentials..."
curl -k -s -u root:new-pass https://172.24.0.3/redfish/v1/AccountService/Accounts/3

echo ""
echo "Stopping the background server..."
kill $SERVER_PID