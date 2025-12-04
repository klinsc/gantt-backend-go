# Backend for SVAR Gantt Chart

### How to start

- create config.yml with DB access config

```yaml
db:
  path: db.sqlite
  resetonstart: true
server:
  url: "http://localhost:8080"
  port: ":8080"
  cors:
    - "*"
```

- start the backend

```shell script
go build
./gantt-backend-go
```
