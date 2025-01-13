'use client'
import Link from "next/link"
import { useRouter } from "next/navigation"
export default function Back () {
    const router = useRouter()

    return (
        <div className="w-24 my-8 p-4 border border-[var(--foreground)] cursor-pointer">
          <div className="text-center" onClick={() => router.back()}>Назад</div>
        </div>
    )
}