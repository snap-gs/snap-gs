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

char *priv = NULL;
char *pub1 = NULL;
char *pub2 = NULL;

ssize_t (*send_next)(int, const void *, size_t, int);

void _init(void) {
    const char *err;
    priv = getenv("SNAPGS_BIND_PRIVATE");
    pub1 = getenv("SNAPGS_BIND_PUBLIC1");
    pub2 = getenv("SNAPGS_BIND_PUBLIC2");
    send_next = dlsym(RTLD_NEXT, "send");
    if ((err = dlerror()) != NULL) {
        fprintf(stderr, "preload: dlerror: %s\n", err);
        exit(99);
    }
    if (priv == NULL) {
        fprintf(stderr, "preload: error: SNAPGS_BIND_PRIVATE\n");
        exit(100);
    }
    if (pub1 == NULL) {
        fprintf(stderr, "preload: error: SNAPGS_BIND_PUBLIC1\n");
        exit(101);
    }
    if (pub2 == NULL) {
        fprintf(stderr, "preload: error: SNAPGS_BIND_PUBLIC2\n");
        exit(102);
    }
}

ssize_t send(int fd, const void *buf, size_t size, int flags) {
    if (size < 45) {
        return send_next(fd, buf, size, flags);
    }
    int offset = -1;
    int len = strlen(priv);
    char *data = (char*) buf;
    for (int i = size-45, j = 0; i < size-len-9; i++) {
        for (j = 0; j < len; j++) {
            if (data[i+j] != priv[j]) {
                break;
            }
        }
        if (j == len && data[i+j] == '|') {
            offset = i;
            break;
        }
    }
    if (offset == -1) {
        return send_next(fd, buf, size, flags);
    }
    char packet[1024] = {0};
    int len1 = strlen(pub1);
    int len2 = strlen(pub2);
    packet[offset+len1] = '|';
    packet[offset-1] = len1 + len2 + 1;
    memcpy(packet, data, offset-1);
    memcpy(&packet[offset], pub1, len1);
    memcpy(&packet[offset+len1+1], pub2, len2);
    send_next(fd, packet, offset+len1+len2+1, flags);
    fprintf(stderr, "preload: %s|%s (%ld -> %d)\n", pub1, pub2, size, offset+len1+len2+1);
    return size;
}

int main(int argc, char **argv) {
    return 0;
}
