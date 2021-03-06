#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <arpa/inet.h>
#include <netinet/in.h>
#include <dlfcn.h>
#include <errno.h>
#include "preload.c"

unsigned char t1[] = {
    162, 188,   0,   1,   0,   0,  52,  63,
     39,  63, 164, 156,  14,   0,   3,   4,
      0,   0,   0,  60,   0,   0,   0,   1,
    243,   2, 253,   3, 252,  73,   1,   4,
    244,   3,  80, 245,   7,  34,  49,  55,
     50,  46,  51,  49,  46,  52,  55,  46,
     50,  53,  49,  58,  53,  48,  53,  54,
    124,  51,  46,  49,  55,  46,  54,  52,
     46,  49,  48,  58,  53,  48,  53,  54,
};

unsigned char t2[] = {
     57, 111,   0,   1,   0,   0,  19, 193,
     76,  72, 114,  37,  14,   0,   3,   4,
      0,   0,   0,  62,   0,   0,   0,   1,
    243,   2, 253,   3, 252,  73,   1,   4,
    244,   3,  80, 245,   7,  36,  49,  55,
     50,  46,  51,  49,  46,  52,  55,  46,
     50,  53,  49,  58,  53,  48,  53,  54,
    124,  51,  46,  49,  57,  46,  50,  53,
     53,  46,  50,  50,  50,  58,  53,  48,
     53,  54,
};

unsigned char t3[] = {
    240,  40,   0,   2,   0,   0,  25, 174,
     59,  74, 140, 137,  16,   0,   0,   0,
      0,   0,   0,  20,   0,   0,   0,   0,
      0,   0,   0,   1,   1,  44, 106, 190,
     14,   0,   3,   4,   0,   0,   0,  59,
      0,   0,   0,   1, 243,   2, 253,   3,
    252,  73,   1,   4, 244,   3,  80, 245,
      7,  33,  49,  55,  50,  46,  51,  49,
     46,  52,  55,  46,  50,  53,  49,  58,
     53,  48,  53,  54, 124,  51,  46,  49,
     54,  46,  51,  49,  46,  55,  58,  53,
     48,  53,  54,
};

unsigned char t4[] = {
     14,  56,   0,   2,   0,   0, 140, 168,
     41,  84,  63, 152,  16,   0,   0,   0,
      0,   0,   0,  20,   0,   0,   0,   0,
      0,   0,   0,   1,  45,  41,  12, 232,
     14,   0,   3,   4,   0,   0,   0,  60,
      0,   0,   0,   1, 243,   2, 253,   3,
    252,  73,   1,   4, 244,   3,  80, 245,
      7,  34,  49,  55,  50,  46,  51,  49,
     46,  52,  55,  46,  50,  53,  49,  58,
     53,  48,  53,  54, 124,  51,  46,  49,
     55,  46,  54,  52,  46,  49,  48,  58,
     53,  48,  53,  54,
};

unsigned char t5[] = {
     94,  22,   0,   2,   0,   1,   1,   0,
     24, 172, 254,  56,  16,   0,   0,   0,
      0,   0,   0,  20,   0,   0,   0,   0,
      0,   0,   0,   5,  45,  57,  30,  48,
     14,   0,   3,   4,   0,   0,   0,  60,
      0,   0,   0,   5, 243,   2, 253,   3,
    252,  73,   1,   8, 244,   3,  80, 245,
      7,  34,  49,  55,  50,  46,  51,  49,
     46,  52,  55,  46,  50,  53,  49,  58,
     53,  48,  53,  54, 124,  51,  46,  49,
     55,  46,  54,  52,  46,  49,  48,  58,
     53,  48,  53,  54,
};

int main(int argc, char **argv) {
    _init();
    send(1, t1, sizeof(t1), 0);
    send(1, t2, sizeof(t2), 0);
    send(1, t3, sizeof(t3), 0);
    send(1, t4, sizeof(t4), 0);
    send(1, t5, sizeof(t5), 0);
    return 0;
}
