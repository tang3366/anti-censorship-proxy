package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

type Config struct {
	Key      []byte
	Addr     string
	IsServer bool
}

func Encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func Decrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func EncodeRequest(targetAddr string, port uint16) ([]byte, error) {
	buf := new(bytes.Buffer)
	ip := net.ParseIP(targetAddr).To4()
	if ip != nil {
		buf.WriteByte(1)
		buf.Write(ip)
	} else {
		buf.WriteByte(3)
		buf.WriteByte(byte(len(targetAddr)))
		buf.Write([]byte(targetAddr))
	}
	binary.Write(buf, binary.BigEndian, port)
	return buf.Bytes(), nil
}

func Client(cfg *Config, targetAddr string, port uint16, payload string) {
	conn, err := net.Dial("tcp", cfg.Addr)
	if err != nil {
		fmt.Println("Dial error:", err)
		return
	}
	defer conn.Close()

	req, err := EncodeRequest(targetAddr, port)
	if err != nil {
		fmt.Println("Encode request error:", err)
		return
	}
	salt := make([]byte, 16)
	rand.Read(salt)
	encryptedReq, err := Encrypt(req, cfg.Key)
	if err != nil {
		fmt.Println("Encrypt request error:", err)
		return
	}
	// 确保完整发送
	_, err = conn.Write(salt)
	if err != nil {
		fmt.Println("Write salt error:", err)
		return
	}
	_, err = conn.Write(encryptedReq)
	if err != nil {
		fmt.Println("Write encrypted request error:", err)
		return
	}

	data := []byte(payload)
	dataLen := make([]byte, 2)
	binary.BigEndian.PutUint16(dataLen, uint16(len(data)))
	padding := make([]byte, 10+time.Now().Nanosecond()%20)
	rand.Read(padding)
	plaintext := append(dataLen, append(data, padding...)...)
	salt = make([]byte, 16)
	rand.Read(salt)
	encryptedData, err := Encrypt(plaintext, cfg.Key)
	if err != nil {
		fmt.Println("Encrypt data error:", err)
		return
	}
	_, err = conn.Write(salt)
	if err != nil {
		fmt.Println("Write data salt error:", err)
		return
	}
	_, err = conn.Write(encryptedData)
	if err != nil {
		fmt.Println("Write encrypted data error:", err)
		return
	}
	fmt.Println("Client sent:", payload)
}

func Server(cfg *Config) {
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		fmt.Println("Listen error:", err)
		return
	}
	defer listener.Close()
	fmt.Println("Server listening on", cfg.Addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		go handleConn(conn, cfg.Key)
	}
}

func handleConn(conn net.Conn, key []byte) {
	defer conn.Close()

	// 读取完整数据包的辅助函数
	readFull := func(buf []byte) error {
		_, err := io.ReadFull(conn, buf)
		return err
	}

	salt := make([]byte, 16)
	if err := readFull(salt); err != nil {
		fmt.Printf("Read salt error: %v\n", err)
		return
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("Read request error: %v, bytes read: %d\n", err, n)
		return
	}
	req, err := Decrypt(buf[:n], key)
	if err != nil {
		fmt.Println("Decrypt request error:", err)
		return
	}

	var targetAddr string
	var port uint16
	switch req[0] {
	case 1:
		targetAddr = net.IP(req[1:5]).String()
		port = binary.BigEndian.Uint16(req[5:7])
	case 3:
		addrLen := int(req[1])
		targetAddr = string(req[2 : 2+addrLen])
		port = binary.BigEndian.Uint16(req[2+addrLen : 4+addrLen])
	default:
		fmt.Println("Unsupported address type")
		return
	}
	fmt.Printf("Target: %s:%d\n", targetAddr, port)

	if err := readFull(salt); err != nil {
		fmt.Printf("Read data salt error: %v\n", err)
		return
	}
	n, err = conn.Read(buf)
	if err != nil {
		fmt.Printf("Read data error: %v, bytes read: %d\n", err, n)
		return
	}
	data, err := Decrypt(buf[:n], key)
	if err != nil {
		fmt.Println("Decrypt data error:", err)
		return
	}
	dataLen := binary.BigEndian.Uint16(data[:2])
	payload := data[2 : 2+dataLen]
	fmt.Println("Server received:", string(payload))
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: proxy <server|client> <address> [target_addr] [port] [payload]")
		return
	}

	mode := os.Args[1]
	addr := os.Args[2]
	key := []byte("my-secret-key-12")

	cfg := &Config{
		Key:  key,
		Addr: addr,
	}

	if mode == "server" {
		cfg.IsServer = true
		go Server(cfg)
		select {}
	} else if mode == "client" && len(os.Args) == 6 {
		targetAddr := os.Args[3]
		var port uint16
		fmt.Sscanf(os.Args[4], "%d", &port)
		payload := os.Args[5]
		Client(cfg, targetAddr, port, payload)
	} else {
		fmt.Println("Invalid arguments for client mode")
	}
}
