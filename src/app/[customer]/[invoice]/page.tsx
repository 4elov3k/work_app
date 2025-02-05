import FormCus from "@/app/components/components/form"
import Back from "@/app/components/components/routeBack"
import Link from "next/link"

export default async function Page({
    params
  }: {
    params: Promise<{ invoice: string }>
  }) {

    return (
        
        <div className="wrapper">
                <nav className="flex gap-8"> 
                <Back/>
                <div className="w-24 my-8 p-4 border border-[var(--foreground)] cursor-pointer">
                    <div className="text-center"><Link href={'/'}>Главная</Link></div>
                </div>
                </nav>
                
            


            <FormCus id={(await params).invoice}/>
        </div>
    )

  }