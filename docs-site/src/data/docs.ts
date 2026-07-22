import type { SiteConfig } from "zuedocs/types";

export const siteConfig = {
  name: "cricinfo",
  strapline: "CLI documentation",
  description:
    "Documentation for cricinfo, a Go-powered Cricinfo command line interface for live matches, scorecards, player and team data, league traversal, and cricket analysis workflows.",
  repoUrl: "https://github.com/amxv/cricinfo-cli",
  accentColor: "#047857",
  accentColorDark: "#34d399",
  footerSections: [
    {
      title: "cricinfo",
      text: "A fast Go CLI for exploring Cricinfo data from a terminal, scripts, and agent workflows."
    },
    {
      title: "Install",
      text: "Published on npm as cricinfo-cli-go and installed as the cricinfo command."
    },
    {
      title: "Repository",
      linkPrefix: "Source: ",
      linkHref: "https://github.com/amxv/cricinfo-cli",
      linkLabel: "github.com/amxv/cricinfo-cli"
    }
  ]
} satisfies SiteConfig;

export const docCategories = [
  "Start",
  "Commands",
  "Automation",
  "Operations"
] as const;

export const primaryNav = [
  { href: "/docs", label: "Docs" },
  { href: "/docs/quickstart", label: "Quickstart" },
  { href: siteConfig.repoUrl, label: "GitHub", external: true }
];
