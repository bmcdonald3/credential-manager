#!/bin/bash

# Define the port
export PORT=8090

echo "🚀 Starting Credential Manager Service on port $PORT in the background..."
go build -o temp_demo_server ./cmd/server
./temp_demo_server serve --port=$PORT --database-url="file:data.db?cache=shared&_fk=1" &
SERVER_PID=$!

# Give the server a few seconds to boot up
sleep 3

echo -e "\n--------------------------------------------------"
echo "🔐 STEP 1: Rotating password from 'initial0' to 'DemoPass123!'"
echo "--------------------------------------------------"
curl -s -X POST http://127.0.0.1:$PORT/bmccredentials/ \
  -H "Content-Type: application/json" \
  -d '{
    "metadata":{"name":"rotate-to-demo"},
    "spec":{
      "targetAddress":"172.24.0.3",
      "currentUsername":"root",
      "currentPassword":"initial0",
      "targetAccount":"3",
      "newPassword":"DemoPass123!"
    }
  }' > /dev/null

echo -e "\n⏳ Waiting 5 seconds for Redfish reconciliation to complete..."
sleep 5

echo -e "\n📊 Current Database State (Look for rotationSucceeded: true):"
curl -s http://127.0.0.1:$PORT/bmccredentials

echo -e "\n\n--------------------------------------------------"
echo "↩️ STEP 2: Rotating password BACK to 'initial0'"
echo "--------------------------------------------------"
curl -s -X POST http://127.0.0.1:$PORT/bmccredentials/ \
  -H "Content-Type: application/json" \
  -d '{
    "metadata":{"name":"rotate-to-initial"},
    "spec":{
      "targetAddress":"172.24.0.3",
      "currentUsername":"root",
      "currentPassword":"DemoPass123!",
      "targetAccount":"3",
      "newPassword":"initial0"
    }
  }' > /dev/null

echo -e "\n⏳ Waiting 5 seconds for Redfish reconciliation to complete..."
sleep 5

echo -e "\n📊 Current Database State (Look for the second successful rotation):"
curl -s http://127.0.0.1:$PORT/bmccredentials

echo -e "\n\n🛑 Shutting down the local server..."
kill $SERVER_PID
echo "Demo complete!"
