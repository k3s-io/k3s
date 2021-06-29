/* ==========================================================================
 * setproctitle.h - Linux/Darwin setproctitle.
 * --------------------------------------------------------------------------
 * Copyright (C) 2010  William Ahern
 * Copyright (C) 2013  Salvatore Sanfilippo
 * Copyright (C) 2013  Stam He
 * Copyright (C) 2013  Erik Dubbelboer
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to permit
 * persons to whom the Software is furnished to do so, subject to the
 * following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
 * OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN
 * NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
 * DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
 * OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
 * USE OR OTHER DEALINGS IN THE SOFTWARE.
 * ==========================================================================
 */
#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif

#include <stddef.h> // NULL size_t
#include <stdarg.h> // va_list va_start va_end
#include <stdlib.h> // malloc(3) setenv(3) clearenv(3) setproctitle(3) getprogname(3)
#include <stdio.h>  // vsnprintf(3) snprintf(3)
#include <string.h> // strlen(3) strdup(3) memset(3) memcpy(3)
#include <errno.h>  /* program_invocation_name program_invocation_short_name */
#include <sys/param.h> /* os versions */
#include <sys/types.h> /* freebsd setproctitle(3) */
#include <unistd.h>    /* freebsd setproctitle(3) */

#if !defined(HAVE_SETPROCTITLE)
#if (__NetBSD__ || __FreeBSD__ || __OpenBSD__)
#define HAVE_SETPROCTITLE 1
#if (__FreeBSD__ && __FreeBSD_version > 1200000)
#define HAVE_SETPROCTITLE_FAST 1
#else
#define HAVE_SETPROCTITLE_FAST 0
#endif
#else
#define HAVE_SETPROCTITLE 0
#define HAVE_SETPROCTITLE_FAST 0
#endif
#endif


#if HAVE_SETPROCTITLE
#define HAVE_SETPROCTITLE_REPLACEMENT 0
#elif (defined __linux || defined __APPLE__)
#define HAVE_SETPROCTITLE_REPLACEMENT 1
#else
#define HAVE_SETPROCTITLE_REPLACEMENT 0
#endif


#if HAVE_SETPROCTITLE_REPLACEMENT

#ifndef SPT_MAXTITLE
#define SPT_MAXTITLE 255
#endif


extern char **environ;


static struct {
  // Original value.
  const char *arg0;

  // First enviroment variable.
  char* env0;

  // Title space available.
  char* base;
  char* end;

  // Pointer to original nul character within base.
  char *nul;

  int reset;
} SPT;


#ifndef SPT_MIN
#define SPT_MIN(a, b) (((a) < (b))? (a) : (b))
#endif


static inline size_t spt_min(size_t a, size_t b) {
  return SPT_MIN(a, b);
}

static char **spt_find_argv_from_env(int argc, char *arg0) {
    int i;
    char **buf = NULL;
    char *ptr;
    char *limit;

    if (!(buf = (char **)malloc((argc + 1) * sizeof(char *)))) {
       return NULL;
    }
    buf[argc] = NULL;

    // Walk back from environ until you find argc-1 null-terminated strings.
    // Don't look for argv[0] as it's probably not preceded by 0.
    ptr = SPT.env0;
    limit = ptr - 8192; // TODO: empiric limit: should use MAX_ARG
    --ptr;
    for (i = argc - 1; i >= 1; --i) {
        if (*ptr) {
          return NULL;
        }
        --ptr;
        while (*ptr && ptr > limit) { --ptr; }
        if (ptr <= limit) {
          return NULL;
        }
        buf[i] = (ptr + 1);
    }

    // The first arg has not a zero in front. But what we have is reliable
    // enough (modulo its encoding). Check if it is exactly what found.
    //
    // The check is known to fail on OS X with locale C if there are
    // non-ascii characters in the executable path.
    ptr -= strlen(arg0);

    if (ptr <= limit) {
       return NULL;
    }
    if (strcmp(ptr, arg0)) {
       return NULL;
    }

    buf[0] = ptr;
    return buf;
}


static int spt_init1() {
  // Store a pointer to the first environment variable since go
  // will overwrite environment.
  SPT.env0 = environ[0];

  return 2;
}

static int spt_fast_init1() {
  return 0;
}


static void spt_init2(int argc, char *arg0) {
  char **argv = spt_find_argv_from_env(argc, arg0);
  char **envp = &SPT.env0;
  char *base, *end, *nul, *tmp;
  int i;

  if (!argv) {
    return;
  }

  if (!(base = argv[0]))
    return;

  nul = &base[strlen(base)];
  end = nul + 1;

  for (i = 0; i < argc || (i >= argc && argv[i]); i++) {
    if (!argv[i] || argv[i] < end)
      continue;

    end = argv[i] + strlen(argv[i]) + 1;
  }

  for (i = 0; envp[i]; i++) {
    if (envp[i] < end)
      continue;

    end = envp[i] + strlen(envp[i]) + 1;
  }

  if (!(SPT.arg0 = strdup(argv[0])))
    return;

#if __GLIBC__
  if (!(tmp = strdup(program_invocation_name)))
    return;

  program_invocation_name = tmp;

  if (!(tmp = strdup(program_invocation_short_name)))
    return;

  program_invocation_short_name = tmp;
#elif __APPLE__
  if (!(tmp = strdup(getprogname())))
    return;

  setprogname(tmp);
#endif

  memset(base, 0, end - base);

  SPT.nul  = nul;
  SPT.base = base;
  SPT.end  = end;

  return;
}


static void setproctitle(const char *fmt, ...) {
  char buf[SPT_MAXTITLE + 1]; // Use buffer in case argv[0] is passed.
  va_list ap;
  char *nul;
  int len;

  if (!SPT.base)
    return;

  if (fmt) {
    va_start(ap, fmt);
    len = vsnprintf(buf, sizeof buf, fmt, ap);
    va_end(ap);
  } else {
    len = snprintf(buf, sizeof buf, "%s", SPT.arg0);
  }

  if (len <= 0) {
    return;
  }

  if (!SPT.reset) {
    memset(SPT.base, 0, SPT.end - SPT.base);
    SPT.reset = 1;
  } else {
    memset(SPT.base, 0, spt_min(sizeof buf, SPT.end - SPT.base));
  }

  len = spt_min(len, spt_min(sizeof buf, SPT.end - SPT.base) - 1);
  memcpy(SPT.base, buf, len);
  nul = &SPT.base[len];

  if (nul < SPT.nul) {
    memset(nul, ' ', SPT.nul - nul);
  } else if (nul == SPT.nul && &nul[1] < SPT.end) {
    *SPT.nul = ' ';
    *++nul = '\0';
  }
}


#else // HAVE_SETPROCTITLE_REPLACEMENT

static int spt_init1() {
#if HAVE_SETPROCTITLE
  return 1;
#else
  return 0;
#endif
}

static int spt_fast_init1() {
#if HAVE_SETPROCTITLE_FAST
  return 1;
#else
  return 0;
#endif
}

static void spt_init2(int argc, char *arg0) {
  (void)argc;
  (void)arg0;
}

#endif // HAVE_SETPROCTITLE_REPLACEMENT



static void spt_setproctitle(const char *title) {
#if HAVE_SETPROCTITLE || HAVE_SETPROCTITLE_REPLACEMENT
  setproctitle("%s", title);
#else
  (void)title;
#endif
}

static void spt_setproctitle_fast(const char *title) {
#if HAVE_SETPROCTITLE_FAST
  setproctitle_fast("%s", title);
#else
  (void)title;
#endif
}

