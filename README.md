# QuACK

## Spec Summary

This document provides a concise summary of the QuACK specification. For more details and clarifications, please refer to the full spec.

### HTTP Messages

QuACK follows the general HTTP message format and includes some additional specifications:

- Supported HTTP version: `HTTP/1.1`
- Supported request method: `GET`
- Supported response statuses:
  - `200 OK`
  - `400 Bad Request`
  - `404 Not Found`
- Required request headers:
  - `Host` (mandatory)
  - `Connection` (optional; `Connection: close` has special meaning affecting server logic)
  - Other headers are allowed but do not affect server logic
- Required response headers:
  - `Date` (mandatory)
  - `Last-Modified` (mandatory for a `200` response)
  - `Content-Type` (mandatory for a `200` response)
  - `Content-Length` (mandatory for a `200` response)
  - `Connection: close` (mandatory for `Connection: close` requests or `400` responses)
  - Response headers should be written in sorted order to simplify testing
  - Response headers should be in 'canonical form', meaning the first letter and any letter after a hyphen should be uppercase, with all other letters lowercase.

### Server Logic
# HTTP Response Handling

## 200 OK Response
A `200` response should be sent when:
- A valid request is received, and the requested file is found.

## 404 Not Found Response
A `404` response should be sent when:
- A valid request is received, but the requested file is not found or is not located under the document root.

## 400 Bad Request Response
A `400` response should be sent when:
- An invalid request is received.
- A timeout occurs, and only a partial request is received.

## Connection Handling
The connection should be closed under the following circumstances:
- When a timeout occurs and no partial request is received.
- When EOF (end of file) occurs.
- After a `400` response is sent.
- After processing a valid request with a `Connection: close` header.

## Timeout Management
The timeout should be updated when:
- Attempting to read a new request.

## Timeout Value
The timeout value is set to 5 seconds.

## Usage

The source code for tools needed to interact with QuACK can be found in the `cmd` directory. The following commands are available:

1) `make fetch` - A tool that allows you to construct custom responses and send them to your server. Please refer to the README in the `fetch` directory for more details.

2) `make quackquack` - Starts your implementation of QuACK.
