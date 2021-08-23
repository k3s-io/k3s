// +build linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package seccomp

import (
	"runtime"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func arches() []specs.Arch {
	switch runtime.GOARCH {
	case "amd64":
		return []specs.Arch{specs.ArchX86_64, specs.ArchX86, specs.ArchX32}
	case "arm64":
		return []specs.Arch{specs.ArchARM, specs.ArchAARCH64}
	case "mips64":
		return []specs.Arch{specs.ArchMIPS, specs.ArchMIPS64, specs.ArchMIPS64N32}
	case "mips64n32":
		return []specs.Arch{specs.ArchMIPS, specs.ArchMIPS64, specs.ArchMIPS64N32}
	case "mipsel64":
		return []specs.Arch{specs.ArchMIPSEL, specs.ArchMIPSEL64, specs.ArchMIPSEL64N32}
	case "mipsel64n32":
		return []specs.Arch{specs.ArchMIPSEL, specs.ArchMIPSEL64, specs.ArchMIPSEL64N32}
	case "s390x":
		return []specs.Arch{specs.ArchS390, specs.ArchS390X}
	default:
		return []specs.Arch{}
	}
}

// DefaultProfile defines the allowed syscalls for the default seccomp profile.
func DefaultProfile(sp *specs.Spec) *specs.LinuxSeccomp {
	syscalls := []specs.LinuxSyscall{
		{
			Names: []string{
				"accept",
				"accept4",
				"access",
				"adjtimex",
				"alarm",
				"bind",
				"brk",
				"capget",
				"capset",
				"chdir",
				"chmod",
				"chown",
				"chown32",
				"clock_adjtime",
				"clock_adjtime64",
				"clock_getres",
				"clock_getres_time64",
				"clock_gettime",
				"clock_gettime64",
				"clock_nanosleep",
				"clock_nanosleep_time64",
				"close",
				"close_range",
				"connect",
				"copy_file_range",
				"creat",
				"dup",
				"dup2",
				"dup3",
				"epoll_create",
				"epoll_create1",
				"epoll_ctl",
				"epoll_ctl_old",
				"epoll_pwait",
				"epoll_pwait2",
				"epoll_wait",
				"epoll_wait_old",
				"eventfd",
				"eventfd2",
				"execve",
				"execveat",
				"exit",
				"exit_group",
				"faccessat",
				"faccessat2",
				"fadvise64",
				"fadvise64_64",
				"fallocate",
				"fanotify_mark",
				"fchdir",
				"fchmod",
				"fchmodat",
				"fchown",
				"fchown32",
				"fchownat",
				"fcntl",
				"fcntl64",
				"fdatasync",
				"fgetxattr",
				"flistxattr",
				"flock",
				"fork",
				"fremovexattr",
				"fsetxattr",
				"fstat",
				"fstat64",
				"fstatat64",
				"fstatfs",
				"fstatfs64",
				"fsync",
				"ftruncate",
				"ftruncate64",
				"futex",
				"futex_time64",
				"futimesat",
				"getcpu",
				"getcwd",
				"getdents",
				"getdents64",
				"getegid",
				"getegid32",
				"geteuid",
				"geteuid32",
				"getgid",
				"getgid32",
				"getgroups",
				"getgroups32",
				"getitimer",
				"getpeername",
				"getpgid",
				"getpgrp",
				"getpid",
				"getppid",
				"getpriority",
				"getrandom",
				"getresgid",
				"getresgid32",
				"getresuid",
				"getresuid32",
				"getrlimit",
				"get_robust_list",
				"getrusage",
				"getsid",
				"getsockname",
				"getsockopt",
				"get_thread_area",
				"gettid",
				"gettimeofday",
				"getuid",
				"getuid32",
				"getxattr",
				"inotify_add_watch",
				"inotify_init",
				"inotify_init1",
				"inotify_rm_watch",
				"io_cancel",
				"ioctl",
				"io_destroy",
				"io_getevents",
				"io_pgetevents",
				"io_pgetevents_time64",
				"ioprio_get",
				"ioprio_set",
				"io_setup",
				"io_submit",
				"io_uring_enter",
				"io_uring_register",
				"io_uring_setup",
				"ipc",
				"kill",
				"lchown",
				"lchown32",
				"lgetxattr",
				"link",
				"linkat",
				"listen",
				"listxattr",
				"llistxattr",
				"_llseek",
				"lremovexattr",
				"lseek",
				"lsetxattr",
				"lstat",
				"lstat64",
				"madvise",
				"membarrier",
				"memfd_create",
				"mincore",
				"mkdir",
				"mkdirat",
				"mknod",
				"mknodat",
				"mlock",
				"mlock2",
				"mlockall",
				"mmap",
				"mmap2",
				"mprotect",
				"mq_getsetattr",
				"mq_notify",
				"mq_open",
				"mq_timedreceive",
				"mq_timedreceive_time64",
				"mq_timedsend",
				"mq_timedsend_time64",
				"mq_unlink",
				"mremap",
				"msgctl",
				"msgget",
				"msgrcv",
				"msgsnd",
				"msync",
				"munlock",
				"munlockall",
				"munmap",
				"nanosleep",
				"newfstatat",
				"_newselect",
				"open",
				"openat",
				"openat2",
				"pause",
				"pidfd_open",
				"pidfd_send_signal",
				"pipe",
				"pipe2",
				"poll",
				"ppoll",
				"ppoll_time64",
				"prctl",
				"pread64",
				"preadv",
				"preadv2",
				"prlimit64",
				"pselect6",
				"pselect6_time64",
				"pwrite64",
				"pwritev",
				"pwritev2",
				"read",
				"readahead",
				"readlink",
				"readlinkat",
				"readv",
				"recv",
				"recvfrom",
				"recvmmsg",
				"recvmmsg_time64",
				"recvmsg",
				"remap_file_pages",
				"removexattr",
				"rename",
				"renameat",
				"renameat2",
				"restart_syscall",
				"rmdir",
				"rseq",
				"rt_sigaction",
				"rt_sigpending",
				"rt_sigprocmask",
				"rt_sigqueueinfo",
				"rt_sigreturn",
				"rt_sigsuspend",
				"rt_sigtimedwait",
				"rt_sigtimedwait_time64",
				"rt_tgsigqueueinfo",
				"sched_getaffinity",
				"sched_getattr",
				"sched_getparam",
				"sched_get_priority_max",
				"sched_get_priority_min",
				"sched_getscheduler",
				"sched_rr_get_interval",
				"sched_rr_get_interval_time64",
				"sched_setaffinity",
				"sched_setattr",
				"sched_setparam",
				"sched_setscheduler",
				"sched_yield",
				"seccomp",
				"select",
				"semctl",
				"semget",
				"semop",
				"semtimedop",
				"semtimedop_time64",
				"send",
				"sendfile",
				"sendfile64",
				"sendmmsg",
				"sendmsg",
				"sendto",
				"setfsgid",
				"setfsgid32",
				"setfsuid",
				"setfsuid32",
				"setgid",
				"setgid32",
				"setgroups",
				"setgroups32",
				"setitimer",
				"setpgid",
				"setpriority",
				"setregid",
				"setregid32",
				"setresgid",
				"setresgid32",
				"setresuid",
				"setresuid32",
				"setreuid",
				"setreuid32",
				"setrlimit",
				"set_robust_list",
				"setsid",
				"setsockopt",
				"set_thread_area",
				"set_tid_address",
				"setuid",
				"setuid32",
				"setxattr",
				"shmat",
				"shmctl",
				"shmdt",
				"shmget",
				"shutdown",
				"sigaltstack",
				"signalfd",
				"signalfd4",
				"sigprocmask",
				"sigreturn",
				"socket",
				"socketcall",
				"socketpair",
				"splice",
				"stat",
				"stat64",
				"statfs",
				"statfs64",
				"statx",
				"symlink",
				"symlinkat",
				"sync",
				"sync_file_range",
				"syncfs",
				"sysinfo",
				"tee",
				"tgkill",
				"time",
				"timer_create",
				"timer_delete",
				"timer_getoverrun",
				"timer_gettime",
				"timer_gettime64",
				"timer_settime",
				"timer_settime64",
				"timerfd_create",
				"timerfd_gettime",
				"timerfd_gettime64",
				"timerfd_settime",
				"timerfd_settime64",
				"times",
				"tkill",
				"truncate",
				"truncate64",
				"ugetrlimit",
				"umask",
				"uname",
				"unlink",
				"unlinkat",
				"utime",
				"utimensat",
				"utimensat_time64",
				"utimes",
				"vfork",
				"vmsplice",
				"wait4",
				"waitid",
				"waitpid",
				"write",
				"writev",
			},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		{
			Names:  []string{"personality"},
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				{
					Index: 0,
					Value: 0x0,
					Op:    specs.OpEqualTo,
				},
			},
		},
		{
			Names:  []string{"personality"},
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				{
					Index: 0,
					Value: 0x0008,
					Op:    specs.OpEqualTo,
				},
			},
		},
		{
			Names:  []string{"personality"},
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				{
					Index: 0,
					Value: 0x20000,
					Op:    specs.OpEqualTo,
				},
			},
		},
		{
			Names:  []string{"personality"},
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				{
					Index: 0,
					Value: 0x20008,
					Op:    specs.OpEqualTo,
				},
			},
		},
		{
			Names:  []string{"personality"},
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				{
					Index: 0,
					Value: 0xffffffff,
					Op:    specs.OpEqualTo,
				},
			},
		},
	}

	s := &specs.LinuxSeccomp{
		DefaultAction: specs.ActErrno,
		Architectures: arches(),
		Syscalls:      syscalls,
	}

	// include by arch
	switch runtime.GOARCH {
	case "ppc64le":
		s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
			Names: []string{
				"sync_file_range2",
			},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		})
	case "arm", "arm64":
		s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
			Names: []string{
				"arm_fadvise64_64",
				"arm_sync_file_range",
				"sync_file_range2",
				"breakpoint",
				"cacheflush",
				"set_tls",
			},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		})
	case "amd64":
		s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
			Names: []string{
				"arch_prctl",
				"modify_ldt",
			},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		})
	case "386":
		s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
			Names: []string{
				"modify_ldt",
			},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		})
	case "s390", "s390x":
		s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
			Names: []string{
				"s390_pci_mmio_read",
				"s390_pci_mmio_write",
				"s390_runtime_instr",
			},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		})
	}

	admin := false
	for _, c := range sp.Process.Capabilities.Bounding {
		switch c {
		case "CAP_DAC_READ_SEARCH":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names:  []string{"open_by_handle_at"},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_ADMIN":
			admin = true
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names: []string{
					"bpf",
					"clone",
					"fanotify_init",
					"fsconfig",
					"fsmount",
					"fsopen",
					"fspick",
					"lookup_dcookie",
					"mount",
					"move_mount",
					"name_to_handle_at",
					"open_tree",
					"perf_event_open",
					"quotactl",
					"setdomainname",
					"sethostname",
					"setns",
					"syslog",
					"umount",
					"umount2",
					"unshare",
				},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_BOOT":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names:  []string{"reboot"},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_CHROOT":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names:  []string{"chroot"},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_MODULE":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names: []string{
					"delete_module",
					"init_module",
					"finit_module",
				},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_PACCT":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names:  []string{"acct"},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_PTRACE":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names: []string{
					"kcmp",
					"pidfd_getfd",
					"process_madvise",
					"process_vm_readv",
					"process_vm_writev",
					"ptrace",
				},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_RAWIO":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names: []string{
					"iopl",
					"ioperm",
				},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_TIME":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names: []string{
					"settimeofday",
					"stime",
					"clock_settime",
				},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYS_TTY_CONFIG":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names:  []string{"vhangup"},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		case "CAP_SYSLOG":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names:  []string{"syslog"},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			})
		}
	}

	if !admin {
		switch runtime.GOARCH {
		case "s390", "s390x":
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names: []string{
					"clone",
				},
				Action: specs.ActAllow,
				Args: []specs.LinuxSeccompArg{
					{
						Index:    1,
						Value:    unix.CLONE_NEWNS | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWUSER | unix.CLONE_NEWPID | unix.CLONE_NEWNET | unix.CLONE_NEWCGROUP,
						ValueTwo: 0,
						Op:       specs.OpMaskedEqual,
					},
				},
			})
		default:
			s.Syscalls = append(s.Syscalls, specs.LinuxSyscall{
				Names: []string{
					"clone",
				},
				Action: specs.ActAllow,
				Args: []specs.LinuxSeccompArg{
					{
						Index:    0,
						Value:    unix.CLONE_NEWNS | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWUSER | unix.CLONE_NEWPID | unix.CLONE_NEWNET | unix.CLONE_NEWCGROUP,
						ValueTwo: 0,
						Op:       specs.OpMaskedEqual,
					},
				},
			})
		}
	}

	return s
}
