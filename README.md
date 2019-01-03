# login-detect-golang

Detect suspicious logins based on Geo locations. This is Golang clone of "Superman Detector" https://github.com/rsirimalla/LoginDetect. 

## Steps to run

```bash
git clone git@github.com:rsirimalla/login-detect-golang.git
cd login-detect-golang
go build
./login-detect-golang
```

Access the app from http://localhost:5000/v1

### Login event example

``` bash
#!/bin/bash
curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"username":"demo","ip_address":"206.81.252.6", "event_uuid":"85ad929a-db03-4bf4-9541-8f728fa12e486", "unix_timestamp":151465500}' \
  http://localhost:5000/v1
```


## Things to improve/implement
- Code refactoring
- Better error handling
- Unit tests
- Dockerize (initial version checked-in but haven't tested fully yet)
