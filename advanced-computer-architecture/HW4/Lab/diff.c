#include <emmintrin.h>
#include <stdint.h>
#include <stdio.h>
#include <x86intrin.h>

#include <stdlib.h>

int main() {
    unsigned int var = 0x42;


   _mm_clflush(&var);
    // wait several cycles for clflush to commit
    for (volatile int i = 0; i < 100; i++);
    // memory fence
    _mm_mfence();

    volatile unsigned int junk;
    uint64_t t0Miss = __rdtscp((unsigned int*)&junk); // get current time stamp
    var++;
    uint64_t deltaMiss = __rdtscp((unsigned int*)&junk) - t0Miss;


    uint64_t t0BHit = __rdtscp((unsigned int*)&junk); // get current time stamp
    var++;
    uint64_t deltaHit = __rdtscp((unsigned int*)&junk) - t0BHit;

    printf("Hit: %u, miss: %u\n", deltaHit, deltaMiss);

  return 0;
}
