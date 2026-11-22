package cache

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type Cache interface {
	Get(context.Context, string) ([]byte, bool, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, ...string) error
}

type Noop struct{}

func (Noop) Get(context.Context, string) ([]byte, bool, error)        { return nil, false, nil }
func (Noop) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (Noop) Delete(context.Context, ...string) error                  { return nil }

type Redis struct{ addr string }

func NewRedis(addr string) *Redis { return &Redis{addr: addr} }

var errNilReply = errors.New("redis nil reply")

func (r *Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	v, err := r.do(ctx, "GET", key)
	if errors.Is(err, errNilReply) {
		return nil, false, nil
	}
	return v, err == nil, err
}

func (r *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	_, err := r.do(ctx, "SET", key, string(value), "EX", strconv.FormatInt(int64(ttl/time.Second), 10))
	return err
}

func (r *Redis) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	args := append([]string{"DEL"}, keys...)
	_, err := r.do(ctx, args...)
	return err
}

func (r *Redis) do(ctx context.Context, args ...string) ([]byte, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", r.addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len([]byte(arg)), arg)
	}
	if _, err := conn.Write([]byte(b.String())); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if line == "$-1" {
		return nil, errNilReply
	}
	if strings.HasPrefix(line, "+") || strings.HasPrefix(line, ":") {
		return []byte(line[1:]), nil
	}
	if strings.HasPrefix(line, "-") {
		return nil, errors.New(line[1:])
	}
	if !strings.HasPrefix(line, "$") {
		return nil, fmt.Errorf("unexpected redis reply %q", line)
	}
	length, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, err
	}
	data := make([]byte, length+2)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}
	return data[:length], nil
}
