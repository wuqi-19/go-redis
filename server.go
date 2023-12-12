package main

type GodisServer struct {
	fd      int
	port    int
	db      *GodisDB
	clients map[int]*GodisClient
	aeLoop  *AeLoop
}

type GodisDB struct {
	data   *Dict
	expire *Dict
}

// 全局变量
var server GodisServer
