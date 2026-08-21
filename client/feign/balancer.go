package feign

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
)

// Balancer selects one instance from the list returned by a Discoverer.
type Balancer interface {
	Pick(instances []Instance) (Instance, error)
}

// RoundRobin picks instances in rotation. It is the default balancer.
type RoundRobin struct {
	counter atomic.Uint64
}

// Pick implements Balancer.
func (r *RoundRobin) Pick(instances []Instance) (Instance, error) {
	n := len(instances)
	if n == 0 {
		return Instance{}, fmt.Errorf("feign: no instances to pick")
	}
	if n == 1 {
		return instances[0], nil
	}
	idx := int(r.counter.Add(1)-1) % n
	return instances[idx], nil
}

// First always picks the first instance; useful as a deterministic fallback.
type First struct{}

// Pick implements Balancer.
func (First) Pick(instances []Instance) (Instance, error) {
	if len(instances) == 0 {
		return Instance{}, fmt.Errorf("feign: no instances to pick")
	}
	return instances[0], nil
}

// addr builds the "host:port" base address for an instance.
func (inst Instance) addr() (string, error) {
	if inst.IP == "" {
		return "", fmt.Errorf("feign: instance %q has no IP", inst.ID)
	}
	if inst.Port <= 0 {
		return "", fmt.Errorf("feign: instance %q has invalid port %d", inst.ID, inst.Port)
	}
	return net.JoinHostPort(inst.IP, strconv.Itoa(inst.Port)), nil
}

// resolveURL computes the full request URL for a method path.
// Discovery (if configured) takes precedence over BaseURL.
func (c *Client) resolveURL(path string) (string, error) {
	if c.Discoverer != nil {
		list, err := c.Discoverer.Discover(c.ServiceName)
		if err != nil {
			return "", fmt.Errorf("feign: discover %q: %w", c.ServiceName, err)
		}
		if len(list) == 0 {
			return "", fmt.Errorf("feign: no instance for service %q", c.ServiceName)
		}
		inst, err := c.balancer().Pick(list)
		if err != nil {
			return "", err
		}
		host, err := inst.addr()
		if err != nil {
			return "", err
		}
		return "http://" + host + path, nil
	}
	if c.BaseURL == "" {
		return "", fmt.Errorf("feign: neither Discoverer nor BaseURL is configured")
	}
	return strings.TrimRight(c.BaseURL, "/") + path, nil
}

func (c *Client) balancer() Balancer {
	if c.Balancer != nil {
		return c.Balancer
	}
	c.balOnce.Do(func() {
		c.balancerVal = &RoundRobin{}
	})
	return c.balancerVal
}
