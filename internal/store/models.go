package store

import (
    "time"
)

type User struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

type Cluster struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

type Project struct {
    ID         string    `json:"id"`
    UserID     string    `json:"user_id"`
    ClusterID  string    `json:"cluster_id"`
    Name       string    `json:"name"`
    Namespace  string    `json:"namespace"`
    Suspended  bool      `json:"suspended"`
    CreatedAt  time.Time `json:"created_at"`
}

type UserSpace struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    ClusterID string    `json:"cluster_id"`
    Namespace string    `json:"namespace"`
    CreatedAt time.Time `json:"created_at"`
}

