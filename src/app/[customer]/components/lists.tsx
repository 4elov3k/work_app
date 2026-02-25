"use client"
import ContractsList from "./contractsList"
import ServicesList from "./servicesList"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Folder, Wrench } from "lucide-react"

export default function Lists({ slug }: { slug: string }) {
  return (
    <Tabs defaultValue="contracts" className="w-full">
      <TabsList className="grid w-full max-w-md grid-cols-2">
        <TabsTrigger value="contracts" className="flex items-center gap-2">
          <Folder className="h-4 w-4" />
          Договоры
        </TabsTrigger>
        <TabsTrigger value="services" className="flex items-center gap-2">
          <Wrench className="h-4 w-4" />
          Услуги
        </TabsTrigger>
      </TabsList>

      <TabsContent value="contracts" className="mt-6">
        <ContractsList slug={slug} />
      </TabsContent>

      <TabsContent value="services" className="mt-6">
        <ServicesList />
      </TabsContent>
    </Tabs>
  )
}
