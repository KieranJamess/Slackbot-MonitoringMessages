# Define the endpoint URL
$endpointUrl = "https://0518-82-14-93-81.ngrok.io/message"

# Create a hashtable representing your JSON data
$jsonData = @{
    "channel" = "C05Q0F85EGZ"
    "mentions" = @("UD1QZGTSS")
    "message" = "Message"
    "reminderIntervals" = 2
}

# $jsonData = @{
#     "channel" = "C05Q0F85EGZ"
#     "mentions" = @("UD1QZGTSS")
#     "message" = "Message"
# }

# Convert the hashtable to JSON format
$jsonBody = $jsonData | ConvertTo-Json

# Set headers for the request (optional)
$headers = @{
    "Content-Type" = "application/json"
}

# Send the JSON data to the endpoint using POST request
$response = Invoke-RestMethod -Uri $endpointUrl -Method Post -Headers $headers -Body $jsonBody

# Display the response
$response
