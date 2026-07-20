"use client"

import { useEffect, useRef, useState } from "react"
import type { ReactNode } from "react"
import { ChevronDown, FileCog } from "lucide-react"

import { Button } from "@/components/ui/button"

type DocumentActionsMenuProps = {
  children: ReactNode
}

export default function DocumentActionsMenu({ children }: DocumentActionsMenuProps) {
  const [open, setOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return

    const handlePointerDown = (event: PointerEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setOpen(false)
      }
    }

    document.addEventListener("pointerdown", handlePointerDown)
    return () => document.removeEventListener("pointerdown", handlePointerDown)
  }, [open])

  return (
    <div className="relative" ref={menuRef}>
      <Button type="button" onClick={() => setOpen((value) => !value)} aria-expanded={open} aria-haspopup="menu">
        <FileCog className="mr-2 h-4 w-4" />
        Действия
        <ChevronDown className="ml-2 h-4 w-4" />
      </Button>
      {open && (
        <div
          role="menu"
          className="absolute right-0 z-50 mt-2 w-64 rounded-md border border-gray-200 bg-white p-2 shadow-lg [&_button]:w-full [&_button]:justify-start"
        >
          <div className="grid gap-2">{children}</div>
        </div>
      )}
    </div>
  )
}
