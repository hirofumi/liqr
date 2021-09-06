# liqr

[Liquid](https://shopify.github.io/liquid/) with Additional Tags and Filters

## Examples

### Tags

#### prompt

```
$ liqr <(echo '{%prompt name = "John" %}{{ "Hello, " | append: name }}')
✔ name: John
```

#### select

```
$ liqr <(echo '{%select env = "dev", "prod" %}{{ env }}')
Use the arrow keys to navigate: ↓ ↑ → ←
? env:
  ▸ dev
    prod
```

### Filters

#### bash

```
$ liqr <(echo '{{ "date +%Y%m%d" | bash }}')
20210907
```

```
$ liqr <(echo '{{ "hello" | bash: "sed s/hello/hi/" }}')
hi
```

## References

* https://github.com/manifoldco/promptui
* https://github.com/osteele/liquid
* https://github.com/Shopify/liquid
