import { parseAttributes, parseProductUpdates, parseProducts, toVariantInput } from './parse'

describe('admin parsing', () => {
  it('builds a variant in rupees with image ids expanded', () => {
    const variant = toVariantInput({
      sku: ' KU-1 ',
      attributes: 'size=M, color=Indigo',
      shownPrice: '1999',
      sellingPrice: '1499',
      currency: 'inr',
      stock: '4',
      images: '01HZX0AAAAAAAAAAAAAAAAAAAA, https://cdn.example.com/a.avif',
    })
    expect(variant).toEqual({
      sku: 'KU-1',
      attributes: { size: 'M', color: 'Indigo' },
      shown_price: 1999,
      selling_price: 1499,
      currency: 'INR',
      stock_units: 4,
      image_urls: ['/v1/images/01HZX0AAAAAAAAAAAAAAAAAAAA', 'https://cdn.example.com/a.avif'],
    })
  })

  it('defaults selling price to shown price', () => {
    expect(
      toVariantInput({
        sku: 'a',
        attributes: '',
        shownPrice: '10',
        sellingPrice: '',
        currency: 'INR',
        stock: '',
        images: '',
      }).selling_price,
    ).toBe(10)
  })

  it('rejects bad input', () => {
    expect(() => parseAttributes('size')).toThrow()
    expect(() =>
      toVariantInput({
        sku: 'a',
        attributes: '',
        shownPrice: 'x',
        sellingPrice: '',
        currency: 'INR',
        stock: '1',
        images: '',
      }),
    ).toThrow()
    expect(() =>
      toVariantInput({
        sku: 'a',
        attributes: '',
        shownPrice: '1',
        sellingPrice: '',
        currency: 'INR',
        stock: '1.5',
        images: '',
      }),
    ).toThrow()
  })

  it('accepts arrays or wrapped product lists', () => {
    expect(parseProducts('[{"name":"a","description":"","variants":[]}]')).toHaveLength(1)
    expect(
      parseProducts('{"products":[{"name":"a","description":"","variants":[]}]}'),
    ).toHaveLength(1)
    expect(() => parseProducts('{"x":1}')).toThrow()
    expect(() => parseProductUpdates('[{"name":"a"}]')).toThrow()
  })
})
