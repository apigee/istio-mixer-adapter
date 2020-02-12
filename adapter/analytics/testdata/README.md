The cert files in this directory were created with the following command:

    go run "$(go env GOROOT)/src/crypto/tls/generate_cert.go" --rsa-bits 1024 --host 127.0.0.1,::1,localhost --ca --start-date "Jan 1 00:00:00 2019" --duration=1000000h
