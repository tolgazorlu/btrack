import {
  ArrowRight,
  Brain,
  CalendarSync,
  Clock,
  FileJson,
  FolderKanban,
  Sparkles,
  Terminal,
  Timer,
  Zap,
} from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { docsRoute, gitConfig } from "@/lib/shared";

export const metadata: Metadata = {
  title: { absolute: "btrack — Time tracker for developers" },
  description:
    "A time tracker for developers. Runs in the terminal, stays out of your way — sessions, projects, Pomodoro, and more.",
};

const repoUrl = `https://github.com/${gitConfig.user}/${gitConfig.repo}`;
const releasesUrl = `${repoUrl}/releases/latest`;

const features = [
  {
    icon: Terminal,
    title: "Terminal-native",
    description:
      "Start, note, and stop sessions without leaving your shell. Keyboard-driven flow that matches how you already work.",
  },
  {
    icon: FolderKanban,
    title: "Projects & notes",
    description:
      "Organize time by project with `.btrack` files and attach notes to running or past sessions.",
  },
  {
    icon: Timer,
    title: "Pomodoro built in",
    description:
      "Focus blocks that auto-tag sessions so your history reflects real deep work.",
  },
  {
    icon: Brain,
    title: "AI summaries",
    description:
      "Turn raw sessions into standups and insights when you need to communicate progress.",
  },
  {
    icon: CalendarSync,
    title: "Google Calendar",
    description:
      "Push sessions to your calendar or pull events into tracked time.",
  },
  {
    icon: FileJson,
    title: "Export & shell prompt",
    description:
      "CSV and JSON export plus prompt integration so your status is always visible.",
  },
] as const;

export default function HomePage() {
  return (
    <div className="flex flex-1 flex-col">
      <section className="relative isolate overflow-hidden border-b border-fd-border">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-10 bg-gradient-to-b from-fd-primary/20 via-fd-primary/[0.07] to-transparent"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute -right-24 top-0 -z-10 h-80 w-80 rounded-full bg-fd-primary/15 blur-3xl"
        />
        <div className="mx-auto flex max-w-5xl flex-col items-center px-4 pb-20 pt-16 text-center sm:pt-24">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-fd-border bg-fd-card/80 px-3 py-1 text-xs font-medium text-fd-muted-foreground shadow-sm backdrop-blur-sm">
            <Sparkles className="size-3.5 text-fd-primary" />
            CLI time tracking for developers
          </div>
          <h1 className="bg-gradient-to-br from-fd-foreground via-fd-foreground to-fd-muted-foreground bg-clip-text text-4xl font-bold tracking-tight text-transparent sm:text-5xl md:text-6xl">
            Ship work. Track time.
            <span className="mt-2 block text-3xl font-semibold sm:text-4xl md:text-5xl">
              All from the terminal.
            </span>
          </h1>
          <p className="mt-6 max-w-2xl text-lg text-fd-muted-foreground sm:text-xl">
            <strong className="font-semibold text-fd-foreground">btrack</strong>{" "}
            is a fast, quiet time tracker that lives in your shell — no extra
            browser tab, no context switch.
          </p>
          <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
            <Link
              href={docsRoute}
              className="inline-flex h-11 items-center justify-center gap-2 rounded-lg bg-fd-primary px-6 text-sm font-medium text-fd-primary-foreground shadow-md transition hover:opacity-90"
            >
              Read the docs
              <ArrowRight className="size-4" />
            </Link>
            <a
              href={releasesUrl}
              className="inline-flex h-11 items-center justify-center rounded-lg border border-fd-border bg-fd-card px-6 text-sm font-medium text-fd-foreground transition hover:bg-fd-accent"
            >
              Get a release
            </a>
            <a
              href={repoUrl}
              className="inline-flex h-11 items-center justify-center rounded-lg px-4 text-sm font-medium text-fd-muted-foreground underline-offset-4 hover:text-fd-foreground hover:underline"
            >
              GitHub
            </a>
          </div>
        </div>
      </section>

      <section className="border-b border-fd-border bg-fd-card/30 py-16">
        <div className="mx-auto max-w-5xl px-4">
          <div className="grid gap-8 lg:grid-cols-[1fr_1.1fr] lg:items-center">
            <div>
              <div className="mb-3 flex items-center gap-2 text-fd-primary">
                <Clock className="size-5" />
                <span className="text-sm font-semibold uppercase tracking-wider">
                  Quick start
                </span>
              </div>
              <h2 className="text-2xl font-bold tracking-tight text-fd-foreground sm:text-3xl">
                Three commands to productive tracking
              </h2>
              <p className="mt-3 text-fd-muted-foreground">
                Install from Homebrew, our install script, or{" "}
                <code className="text-fd-foreground">go install</code>
                {" — "}then start a session in seconds.
              </p>
              <ul className="mt-6 space-y-3 text-sm text-fd-muted-foreground">
                <li className="flex gap-2">
                  <Zap className="mt-0.5 size-4 shrink-0 text-fd-primary" />
                  <span>
                    See the full command reference and configuration in{" "}
                    <Link
                      href={docsRoute}
                      className="font-medium text-fd-primary hover:underline"
                    >
                      the documentation
                    </Link>
                    .
                  </span>
                </li>
              </ul>
            </div>
            <div className="rounded-xl border border-fd-border bg-fd-background shadow-lg shadow-fd-primary/5">
              <div className="flex items-center gap-2 border-b border-fd-border px-4 py-3">
                <span className="size-3 rounded-full bg-red-500/80" />
                <span className="size-3 rounded-full bg-amber-500/80" />
                <span className="size-3 rounded-full bg-emerald-500/80" />
                <span className="ml-2 font-mono text-xs text-fd-muted-foreground">
                  ~/project
                </span>
              </div>
              <pre className="overflow-x-auto p-4 font-mono text-[13px] leading-relaxed text-fd-foreground sm:text-sm">
                <code>
                  <span className="text-fd-muted-foreground">
                    # start tracking
                  </span>
                  {"\n"}
                  <span className="text-fd-primary">btrack</span>
                  <span>{` s "fix login bug"`}</span>
                  {"\n\n"}
                  <span className="text-fd-muted-foreground"># add a note</span>
                  {"\n"}
                  <span className="text-fd-primary">btrack</span>
                  <span>{` n "found JWT clock skew"`}</span>
                  {"\n\n"}
                  <span className="text-fd-muted-foreground">
                    # stop & save
                  </span>
                  {"\n"}
                  <span className="text-fd-primary">btrack</span>
                  <span>{` x -m "fixed clock skew"`}</span>
                </code>
              </pre>
            </div>
          </div>
        </div>
      </section>

      <section className="py-20">
        <div className="mx-auto max-w-5xl px-4">
          <h2 className="text-center text-2xl font-bold tracking-tight text-fd-foreground sm:text-3xl">
            Built for deep work
          </h2>
          <p className="mx-auto mt-3 max-w-xl text-center text-fd-muted-foreground">
            Everything you need to understand where your time went — without
            fighting a heavy UI.
          </p>
          <ul className="mt-14 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {features.map(({ icon: Icon, title, description }) => (
              <li
                key={title}
                className="group rounded-xl border border-fd-border bg-fd-card/50 p-6 transition hover:border-fd-primary/30 hover:shadow-md hover:shadow-fd-primary/5"
              >
                <div className="mb-4 flex size-10 items-center justify-center rounded-lg bg-fd-primary/10 text-fd-primary transition group-hover:bg-fd-primary/15">
                  <Icon className="size-5" />
                </div>
                <h3 className="font-semibold text-fd-foreground">{title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-fd-muted-foreground">
                  {description}
                </p>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="border-t border-fd-border bg-gradient-to-b from-fd-primary/8 to-transparent py-16">
        <div className="mx-auto max-w-3xl px-4 text-center">
          <h2 className="text-xl font-bold tracking-tight text-fd-foreground sm:text-2xl">
            Ready to try btrack?
          </h2>
          <p className="mt-2 text-fd-muted-foreground">
            Open-source under MIT. Star the repo, install a binary, and start
            your first session today.
          </p>
          <div className="mt-8 flex flex-wrap justify-center gap-3">
            <Link
              href={`${docsRoute}/installation`}
              className="inline-flex h-11 items-center justify-center gap-2 rounded-lg bg-fd-primary px-6 text-sm font-medium text-fd-primary-foreground transition hover:opacity-90"
            >
              Installation guide
              <ArrowRight className="size-4" />
            </Link>
            <a
              href={repoUrl}
              className="inline-flex h-11 items-center justify-center rounded-lg border border-fd-border bg-fd-background px-6 text-sm font-medium transition hover:bg-fd-accent"
            >
              View on GitHub
            </a>
          </div>
        </div>
      </section>
    </div>
  );
}
