import Link from "next/link"
import { FileQuestion } from "lucide-react"
import { Button } from "@/components/ui/button"

export default function NotFound() {
  return (
    <div className="container mx-auto flex min-h-[60vh] max-w-xl flex-col items-center justify-center gap-4 px-4 text-center">
      <FileQuestion className="h-12 w-12 text-muted-foreground" />
      <h1 className="text-2xl font-bold tracking-tight">Страница не найдена</h1>
      <p className="text-muted-foreground">
        Документ, контрагент или страница по этой ссылке не найдены — возможно, запись удалена или ссылка устарела.
      </p>
      <Link href="/">
        <Button>На главную</Button>
      </Link>
    </div>
  )
}
