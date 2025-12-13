"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils/cn";
import {
  LayoutDashboard,
  FolderKanban,
  Activity,
  Settings,
  HelpCircle,
} from "lucide-react";

interface SidebarLink {
  href: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
}

const mainLinks: SidebarLink[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/activity", label: "Activity", icon: Activity },
];

const bottomLinks: SidebarLink[] = [
  { href: "/settings", label: "Settings", icon: Settings },
  { href: "/help", label: "Help", icon: HelpCircle },
];

export function Sidebar() {
  const pathname = usePathname();

  const renderLink = (link: SidebarLink) => {
    const Icon = link.icon;
    const isActive = pathname === link.href || pathname.startsWith(`${link.href}/`);

    return (
      <Link
        key={link.href}
        href={link.href}
        className={cn(
          "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-all duration-200",
          isActive
            ? "bg-primary/10 text-primary"
            : "text-surface-400 hover:bg-surface-800/50 hover:text-foreground"
        )}
      >
        <Icon className={cn("h-5 w-5 flex-shrink-0", isActive && "text-primary")} />
        <span>{link.label}</span>
        {isActive && (
          <div className="ml-auto h-1.5 w-1.5 rounded-full bg-primary" />
        )}
      </Link>
    );
  };

  return (
    <aside className="fixed left-0 top-16 bottom-0 z-40 hidden w-64 border-r border-surface-800 bg-surface-950 lg:block">
      <div className="flex h-full flex-col">
        {/* Main navigation */}
        <nav className="flex-1 space-y-1 px-3 py-4">
          <div className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-surface-500">
            Navigation
          </div>
          {mainLinks.map(renderLink)}
        </nav>

        {/* Bottom section */}
        <div className="border-t border-surface-800 px-3 py-4">
          <div className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-surface-500">
            Support
          </div>
          {bottomLinks.map(renderLink)}
        </div>

        {/* Status indicator */}
        <div className="border-t border-surface-800 px-4 py-3">
          <div className="flex items-center gap-2 text-xs text-surface-500">
            <div className="h-2 w-2 rounded-full bg-accent-emerald animate-pulse" />
            <span>All systems operational</span>
          </div>
        </div>
      </div>
    </aside>
  );
}

