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
char send_buf1[46] = {0};
char send_buf2[46] = {0};


ssize_t (*send_next)(int, const void *, size_t, int);


void _init(void) {

    const unsigned char minaddr[] = "0.0.0.0:0";
    const unsigned char maxaddr[] = "000.000.000.000:00000";
    const size_t minlen = sizeof(minaddr)-1;
    const size_t maxlen = sizeof(maxaddr)-1;

    if ((send_next = dlsym(RTLD_NEXT, "send")) == NULL) {
        fprintf(stderr, "preload.c: _init: dlerror: %s\n", dlerror());
        exit(90);
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

}


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
