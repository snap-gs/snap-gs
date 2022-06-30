// gcc -nostartfiles -fpic -shared hack/preload.c -o /tmp/snap-gs-preload.so -ldl -D_GNU_SOURCE


#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <arpa/inet.h>
#include <netinet/in.h>
#include <dlfcn.h>
#include <errno.h>


char *snapgs_lobby_listen = NULL;
char *snapgs_lobby_listen1 = NULL;
char *snapgs_lobby_listen2 = NULL;


// Largest is "\x07\x2b000.000.000.000:00000|000.000.000.000:00000\x00".
size_t send_buf1_size = 0;
size_t send_buf2_size = 0;
unsigned char send_buf1[46] = {0};
unsigned char send_buf2[46] = {0};


ssize_t (*send_next)(int, const void *, size_t, int);
ssize_t (*recv_next)(int, void *, size_t, int);
ssize_t (*recvfrom_next)(int, void *restrict, size_t, int, struct sockaddr *restrict, socklen_t *restrict);


void _init(void) {

    const unsigned char minaddr[] = "0.0.0.0:0";
    const unsigned char maxaddr[] = "000.000.000.000:00000";
    const size_t minlen = sizeof(minaddr)-1;
    const size_t maxlen = sizeof(maxaddr)-1;

    if ((send_next = dlsym(RTLD_NEXT, "send")) == NULL) {
        fprintf(stderr, "preload.c: _init: dlerror: %s\n", dlerror());
        exit(90);
    } else if ((recv_next = dlsym(RTLD_NEXT, "recv")) == NULL) {
        fprintf(stderr, "preload.c: _init: dlerror: %s\n", dlerror());
        exit(91);
    } else if ((recvfrom_next = dlsym(RTLD_NEXT, "recvfrom")) == NULL) {
        fprintf(stderr, "preload.c: _init: dlerror: %s\n", dlerror());
        exit(92);
    }

    if ((snapgs_lobby_listen = getenv("SNAPGS_LOBBY_LISTEN")) == NULL) {
        fprintf(stderr, "preload.c: _init: getenv: SNAPGS_LOBBY_LISTEN\n");
        exit(100);
    } else if (strlen(snapgs_lobby_listen) < minlen || strlen(snapgs_lobby_listen) > maxlen) {
        fprintf(stderr, "preload.c: _init: strlen: SNAPGS_LOBBY_LISTEN: %lu < %lu < %lu\n",
                minlen, strlen(snapgs_lobby_listen), maxlen);
        exit(101);
    }

    if ((snapgs_lobby_listen1 = getenv("SNAPGS_LOBBY_LISTEN1")) == NULL) {
        fprintf(stderr, "preload.c: _init: getenv: SNAPGS_LOBBY_LISTEN1\n");
        exit(110);
    } else if (strlen(snapgs_lobby_listen1) < minlen || strlen(snapgs_lobby_listen1) > maxlen) {
        fprintf(stderr, "preload.c: _init: strlen: SNAPGS_LOBBY_LISTEN1: %lu < %lu < %lu\n",
                minlen, strlen(snapgs_lobby_listen1), maxlen);
        exit(111);
    }

    if ((snapgs_lobby_listen2 = getenv("SNAPGS_LOBBY_LISTEN2")) == NULL) {
        fprintf(stderr, "preload.c: _init: getenv: SNAPGS_LOBBY_LISTEN2\n");
        exit(120);
    } else if (strlen(snapgs_lobby_listen2) < minlen || strlen(snapgs_lobby_listen2) > maxlen) {
        fprintf(stderr, "preload.c: _init: strlen: SNAPGS_LOBBY_LISTEN2: %lu < %lu < %lu\n",
                minlen, strlen(snapgs_lobby_listen2), maxlen);
        exit(121);
    }

    const size_t len = strlen(snapgs_lobby_listen);
    const size_t len1 = strlen(snapgs_lobby_listen1);
    const size_t len2 = strlen(snapgs_lobby_listen2);
    send_buf1_size = 2+len+1+len1, send_buf2_size = 2+len+1+len1;

    memset(&send_buf1, '\0', sizeof(send_buf1));
    memset(&send_buf2, '\0', sizeof(send_buf2));
    memset(&send_buf2[2], '0', len+1+len1);
    memcpy(&send_buf1[2], snapgs_lobby_listen, len);
    memcpy(&send_buf1[2+len+1], snapgs_lobby_listen1, len1);
    memcpy(&send_buf2[2+len+1+len1-len2], snapgs_lobby_listen2, len2);
    send_buf1[0] = 7, send_buf1[1] = len+1+len1, send_buf1[2+len] = '|';
    send_buf2[0] = 7, send_buf2[1] = len+1+len1, send_buf2[2+len+len1-len2] = '|', send_buf2[2+len+len1-len2-2] = ':';

    if (len == 19 && len1 == 19 && len2 == 16)
        // memcpy(&send_buf2[2], "3.33.251.70:27002|fast-001.snap.gs:8978", 39);
        // memcpy(&send_buf2[2], "0.0.0.0:0|18.224.29.212:27002|0.0.0.0:0", 39);
        memcpy(&send_buf2[2], "0.0.0.0:0|0.0.0.0:0|18.224.29.212:27002", 39);

}

ssize_t recv(int fd, void *buf, size_t size, int flags) {
    return recv_next(fd, buf, size, flags);
}

ssize_t recvfrom(int fd, void *restrict buf, size_t len, int flags,
                 struct sockaddr *restrict addr, socklen_t *restrict addrlen) {

    ssize_t size = recvfrom_next(fd, buf, len, flags, addr, addrlen);
    if (size < sizeof(send_buf1) || size == -1)
        return size;

    unsigned char *data = (unsigned char *) buf;
    unsigned char *last = (unsigned char *) buf + size;
    while ((data = memchr(data, 7, last-data)) != NULL) {

        if (last-data < 21)
            break;

        unsigned int ips_size = data[1];
        if (ips_size < 19 || ips_size > 43) {
            if (++data < last)
                continue;

            break;
        }

        unsigned char *p = NULL;
        unsigned char *c = &data[2];

        while (c < data + 2 + ips_size) {
            switch (c[0]) {
                case '|':
                    p = c;
                case '0':
                case '1':
                case '2':
                case '3':
                case '4':
                case '5':
                case '6':
                case '7':
                case '8':
                case '9':
                case ':':
                case '.':
                    c++;
                    continue;
                default:
                    p = NULL;
            }
            break;
        }

        if (p == NULL) {
            if (++data < last)
                continue;

            break;
        }

        unsigned char a1[22] = {0};
        unsigned char a2[22] = {0};

        memcpy(&a1, &data[2], p-data-2);
        memcpy(&a2, p+1, data+1+ips_size-p);

        if (p-data-2 == data+1+ips_size-p) {

            fprintf(stderr, "preload.c: recvfrom <<< \"%s|%s\" (%d)\n", (char*)&a1, (char*)&a2, ips_size);

            memcpy(&data[2], "0000000000000000:0", p-data-2);
            // memcpy(&data[2], &a2, p-data-2);
            // memcpy(p+1, &a1, data+1+ips_size-p);

            unsigned char a3[44] = {0};
            memcpy(&a3, &data[2], ips_size);

            fprintf(stderr, "preload.c: recvfrom >>> \"%s\" (%d)\n", (char*)&a3, ips_size);

        }

        if ((data += 2 + ips_size) >= last)
            break;

    }

    return size;
};

ssize_t send(int fd, const void *buf, size_t size, int flags) {

    if (size < sizeof(send_buf1))
        return send_next(fd, buf, size, flags);

    unsigned char *data = (unsigned char *) buf;
    unsigned char *last = (unsigned char *) buf + size;

    while ((data = memchr(data, send_buf1[0], last-data)) != NULL) {

        if (memcmp(data, send_buf1, send_buf1_size) != 0) {
            if (++data < last)
                continue;

            break;

        }

        fprintf(stderr, "preload.c: send[%lu-%lu+%lu%+d]: %d, %d, \"%s\" (%lu) -> %d, %d, \"%s\" (%lu)\n",
                size, last-data, send_buf1_size, (int)send_buf2_size-(int)send_buf1_size,
                send_buf1[0], send_buf1[1], &send_buf1[2], send_buf1_size,
                send_buf2[0], send_buf2[1], &send_buf2[2], send_buf2_size);

        memcpy(data, send_buf2, send_buf1_size);
        if ((data += send_buf1_size) >= last)
            break;

    }

    return send_next(fd, buf, size, flags);

}
