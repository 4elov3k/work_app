import FormCus from "@/app/components/components/form"
import Back from "@/app/components/components/routeBack"
import Link from "next/link"
import Print from "./components/print"
import AddForm from "./components/addForm"

export default async function Page({
    params
  }: {
    params: Promise<{ invoice: string }>
  }) {


    
    return (
        
        <div className="wrapper">
          <div className="not-print">
              <nav className="flex gap-8"> 
                  <Back/>
                  <div className="w-24 btn my-8 p-4 border border-[var(--foreground)] cursor-pointer">
                      <div className="text-center"><Link href={'/'}>Главная</Link></div>
                  </div>
                  </nav>
                  
              

              <Print/>
              <AddForm id={(await params).invoice}/>
             
          </div>
                
            <FormCus id={(await params).invoice}/>
            
        </div>
    )

  }