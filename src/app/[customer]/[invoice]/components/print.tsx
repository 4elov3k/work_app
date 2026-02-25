"use client"
import { Printer } from "lucide-react"
import { Button } from "@/components/ui/button"

export default function Print () {
    const print = () => window.print();
    
    return(
        <Button onClick={print} variant="default">
            <Printer className="mr-2 h-4 w-4" />
            Печать
        </Button>
    )
}