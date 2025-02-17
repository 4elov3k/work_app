'use client'
import Link from "next/link"
import { useRouter } from "next/navigation"
import React from "react"
export default function Back () {
    const router = useRouter()
    
    return (
        <div className="w-24 btn my-8 p-4 border border-[var(--foreground)] cursor-pointer">
          <button className="text-center" onClick={(event: React.MouseEvent<HTMLButtonElement>) => router.back()}>Назад</button>
          
        </div>
    )
}