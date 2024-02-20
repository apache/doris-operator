# 工行与主仓库不一致的地方
1. CRD使用的v1beta1 版本，需要使用bin下的controller-gen来生成.
2. 主仓库脚本准备使用golang开发替代。目前icbc没办法
3. 脚本中加入添加/etc/hosts的内容逻辑，主要是为了实现hadoop相关域名解析ip的能力.

