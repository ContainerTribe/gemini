package main

/*
#include <stdio.h>
#include <stdlib.h>
#include <fcntl.h>
#include <string.h>

#include <sys/ioctl.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <net/if.h>
#include <error.h>
#include <net/route.h>

int SetIfAddr(char *ifname, char *Ipaddr, char *mask)
{
	int fd;
	int rc;
	struct ifreq ifr;
	struct sockaddr_in *sin;
	struct rtentry rt;

	fd = socket(AF_INET, SOCK_DGRAM, 0);
	if(fd <0)
	{
		perror("socket error");
		return -1;
	}
	memset(&ifr, 0, sizeof(ifr));
	strcpy(ifr.ifr_name, ifname);
	sin = (struct sockaddr_in*)&ifr.ifr_addr;
	sin->sin_family = AF_INET;

	// ip addr
	if(inet_aton(Ipaddr, &(sin->sin_addr)) <0)
	{
		perror("inet_aton error");
		return -2;
	}

	if(ioctl(fd, SIOCSIFADDR, &ifr) <0)
	{
		perror("ioctl SIOCSIFADDR error");
		return -3;
	}

	// netmask
	if(inet_aton(mask, &(sin->sin_addr)) <0)
	{
		perror("inet_aton error");
		return -4;
	}
	if(ioctl(fd, SIOCSIFNETMASK, &ifr) <0)
	{
		perror("ioctl SIOCSIFNETMASK error");
		return -5;
	}

	// up interface
	ifr.ifr_flags |= IFF_UP | IFF_RUNNING;
	if(ioctl(fd, SIOCSIFFLAGS, &ifr) < 0)
	{
		perror("ioctl SIOCSIFFLAGS error");
		return -6;
	}
	close(fd);
	return rc;
}
*/
import "C"

import (
	"errors"
	"fmt"
)

func setIp(ifname string, ipaddr string, netmask string) error {
	code := C.SetIfAddr(C.CString(ifname), C.CString(ipaddr), C.CString(netmask))
	if code != 0 {
		return errors.New(fmt.Sprintf("Set Ip error with code %d", code))
	}
	return nil
}
