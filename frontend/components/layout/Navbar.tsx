"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils/cn";
import { useAuthStore } from "@/lib/store/authStore";
import { LogOut, User, Settings, Menu, X } from "lucide-react";
import { useState } from "react";

export function Navbar() {
  const pathname = usePathname();
  const { user, logout } = useAuthStore();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const navLinks = [
    { href: "/dashboard", label: "Dashboard" },
    { href: "/projects/new", label: "New Project" },
  ];

  const handleLogout = async () => {
    logout();
    window.location.href = "/";
  };

  return (
    <nav className="fixed top-0 left-0 right-0 z-50 border-b border-surface-800 bg-surface-950/80 backdrop-blur-xl">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex h-16 items-center justify-between">
          {/* Logo */}
          <Link href={user ? "/dashboard" : "/"} className="flex items-center gap-3">
            <img
              src="/logo.png"
              alt="NexusDeploy"
              className="h-8 w-8 rounded-lg object-contain"
            />
            <span className="text-lg font-semibold text-foreground">NexusDeploy</span>
          </Link>

          {/* Desktop Navigation */}
          {user && (
            <div className="hidden items-center gap-1 md:flex">
              {navLinks.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  className={cn(
                    "rounded-lg px-4 py-2 text-sm font-medium transition-colors",
                    pathname === link.href
                      ? "bg-surface-800 text-foreground"
                      : "text-surface-400 hover:bg-surface-800/50 hover:text-foreground"
                  )}
                >
                  {link.label}
                </Link>
              ))}
            </div>
          )}

          {/* Right section */}
          <div className="flex items-center gap-4">
            {user ? (
              <>
                {/* User menu */}
                <div className="hidden items-center gap-3 md:flex">
                  <Link
                    href="/settings"
                    className="rounded-lg p-2 text-surface-400 transition-colors hover:bg-surface-800 hover:text-foreground"
                  >
                    <Settings className="h-5 w-5" />
                  </Link>
                  <div className="h-6 w-px bg-surface-700" />
                  <div className="flex items-center gap-3">
                    {user.avatar_url ? (
                      <img
                        src={user.avatar_url}
                        alt={user.username}
                        className="h-8 w-8 rounded-full border border-surface-700"
                      />
                    ) : (
                      <div className="flex h-8 w-8 items-center justify-center rounded-full bg-surface-800">
                        <User className="h-4 w-4 text-surface-400" />
                      </div>
                    )}
                    <span className="text-sm font-medium text-foreground">{user.username}</span>
                  </div>
                  <button
                    onClick={handleLogout}
                    className="rounded-lg p-2 text-surface-400 transition-colors hover:bg-surface-800 hover:text-accent-rose"
                  >
                    <LogOut className="h-5 w-5" />
                  </button>
                </div>

                {/* Mobile menu button */}
                <button
                  onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                  className="rounded-lg p-2 text-surface-400 transition-colors hover:bg-surface-800 hover:text-foreground md:hidden"
                >
                  {mobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
                </button>
              </>
            ) : (
              <Link
                href="/login"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-600"
              >
                Sign In
              </Link>
            )}
          </div>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileMenuOpen && user && (
        <div className="animate-slide-down border-t border-surface-800 bg-surface-950/95 backdrop-blur-xl md:hidden">
          <div className="space-y-1 px-4 py-4">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setMobileMenuOpen(false)}
                className={cn(
                  "block rounded-lg px-4 py-3 text-sm font-medium transition-colors",
                  pathname === link.href
                    ? "bg-surface-800 text-foreground"
                    : "text-surface-400 hover:bg-surface-800/50 hover:text-foreground"
                )}
              >
                {link.label}
              </Link>
            ))}
            <div className="my-2 h-px bg-surface-800" />
            <Link
              href="/settings"
              onClick={() => setMobileMenuOpen(false)}
              className="flex items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium text-surface-400 transition-colors hover:bg-surface-800/50 hover:text-foreground"
            >
              <Settings className="h-4 w-4" />
              Settings
            </Link>
            <button
              onClick={handleLogout}
              className="flex w-full items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium text-surface-400 transition-colors hover:bg-surface-800/50 hover:text-accent-rose"
            >
              <LogOut className="h-4 w-4" />
              Sign Out
            </button>
          </div>
        </div>
      )}
    </nav>
  );
}

