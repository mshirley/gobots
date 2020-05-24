#!/usr/bin/env bash
mkdir -p server/pki
openssl req -x509 -newkey rsa:4096 -keyout server/pki/key.pem -out server/pki/cert.pem -days 365 -nodes

