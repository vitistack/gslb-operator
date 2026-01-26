/*
    - inside the lua script users has access to three global variables:
        - status_code: http status code returned in the http request
        - headers: a table containing all the headers in the response
        - body: response body
    
    this allows the users to customize what a healthy response from the service looks like:
    NOTE: the script must return true/false 
    NOTE: the script is stored in the GSLB - config
*/

-- Check status code only
return status_code >= 200 and status_code < 400

-- Check status and body content
return status_code == 200 and string.find(body, "healthy") ~= nil

-- Check status and header
return status_code == 200 and headers["Content-Type"] == "application/json"

-- Complex check with multiple conditions
local status_ok = status_code >= 200 and status_code < 300
local has_json = headers["Content-Type"] == "application/json"
local valid_body = string.find(body, '"status":"ok"') ~= nil
return status_ok and has_json and valid_body